package main

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"strings"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/gofrs/uuid"
)

var (
	logger = log.New(os.Stdout, "[Pi-Sensor Log Forwarder] ", log.LstdFlags)
)

const (
	logForwarderChannel = "logs/submit"
)

func main() {
	brokerURL := os.Getenv("CLOUDMQTT_URL")
	if brokerURL == "" {
		logger.Fatalln("CLOUDMQTT_URL is required")
	}

	systemdUnit := os.Getenv("LOG_FORWARDER_SYSTEMD_UNIT")
	if systemdUnit == "" {
		logger.Fatalln("LOG_FORWARDER_SYSTEMD_UNIT is required")
	}

	_mqttClient := newMQTTClient(brokerURL)

	ch := make(chan string)
	go tailSystemdLogs(systemdUnit, ch)

	for logs := range ch {
		logLines := strings.Split(strings.Replace(logs, "\n", `\n`, -1), `\n`)
		for _, line := range logLines {
			if line != "" {
				_mqttClient.sendLogs(line)
			}
		}
	}
}

var connectHandler mqtt.OnConnectHandler = func(client mqtt.Client) {
	logger.Println("Connected to MQTT server")
}

var connectLostHandler mqtt.ConnectionLostHandler = func(client mqtt.Client, err error) {
	logger.Fatalf("Connection to MQTT server lost: %v", err)
}

type mqttClient struct {
	client mqtt.Client
}

func newMQTTClient(brokerurl string) mqttClient {
	opts := mqtt.NewClientOptions()
	opts.AddBroker(brokerurl)
	var clientID string
	u, _ := uuid.NewV4()
	clientID = u.String()
	logger.Println("Starting MQTT client with id", clientID)
	opts.SetClientID(clientID)
	opts.OnConnect = connectHandler
	opts.OnConnectionLost = connectLostHandler
	client := mqtt.NewClient(opts)
	if token := client.Connect(); token.Wait() && token.Error() != nil {
		panic(token.Error())
	}
	return mqttClient{
		client,
	}
}

func (c mqttClient) Cleanup() {
	c.client.Disconnect(250)
}

func (c mqttClient) sendLogs(log string) {
	text := fmt.Sprintf("{\"message\":\"%s\"}", log)
	token := c.client.Publish(logForwarderChannel, 0, false, text)
	token.Wait()
}

func tailSystemdLogs(systemdUnit string, ch chan string) {
	cmd := exec.Command("journalctl", "-u", systemdUnit, "-f", "-n 0")
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		log.Fatal(err)
	}

	if err := cmd.Start(); err != nil {
		log.Fatal(err)
	}

	buf := make([]byte, 1024)
	for {
		n, err := stdout.Read(buf)
		if err != nil {
			break
		}

		ch <- string(buf[0:n])
	}
	close(ch)
	if err := cmd.Wait(); err != nil {
		log.Fatal(err)
	}
}
