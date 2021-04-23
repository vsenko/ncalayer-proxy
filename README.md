# ncalayer-proxy
A simple tool to debug communications with NCALayer

Starts a WebSocket proxy that prints all passing messages to console. Without `-no-confirmations` flag suspends all messages until user confirmation.

Requires port forwarding to be configured, currently tested only with Linux `iptables`. Instructions are printed to console during startup.

## Installing

Download latest build from releases.

## Building

```
go build
```