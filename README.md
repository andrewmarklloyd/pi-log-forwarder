# pi-log-forwarder

This tool tails the output of `journalctl` for a given systemd service. The logs are published to a configurable MQTT topic. It is up to a given log server to parse the json logs and store as necessary.
