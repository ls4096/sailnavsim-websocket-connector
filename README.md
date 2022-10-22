# SailNavSim WebSocket Connector

This program exposes a WebSocket interface to provide some data, such as live boat information, made available by a corresponding instance of sailnavsim-core (as seen here: https://8bitbyte.ca/sailnavsim/).

## Dependencies

- Standard Golang build tools

### Tested build/run environments

- Ubuntu 20.04, x86-64
- Debian 9 (Stretch), x86-64

## How to build

`go get`

`go build`

### Run tests

`go test`

## How to run

`./sailnavsim-snsw <listen_port> <connect_port>`

The above command will run the WebSocket Connector program, exposing its WebSocket interface on localhost port `<listen_port>`, and connecting to the running sailnavsim-core simulator program at localhost port `<connect_port>`. While running, the WebSocket endpoint will be available at `http://localhost:<listen_port>/v1/ws`.
