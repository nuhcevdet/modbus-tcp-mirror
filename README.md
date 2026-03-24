# Modbus TCP Mirror

A lightweight Go application that creates an exact replica of a Modbus TCP device. It connects to a source PLC/device as a Modbus master, periodically polls configured register blocks, and re-exposes the same data as a Modbus TCP slave server.

```
┌──────────┐     poll      ┌───────────────┐     serve     ┌──────────┐
│ Source    │◄─────────────│  Modbus TCP    │◄─────────────│  Modbus   │
│ PLC      │  FC01-04      │  Mirror        │  FC01-06,    │  Clients  │
│          │──────────────►│                │  FC0F-10     │           │
└──────────┘   responses   └───────────────┘──────────────►└──────────┘
```

## Features

- **Full register type support**: Holding registers, input registers, coils, and discrete inputs
- **Configurable polling**: Adjustable polling interval, timeouts, and retry logic
- **Multiple register blocks**: Define as many address ranges as needed
- **Write-through support**: Clients can write to the mirror (FC05, FC06, FC0F, FC10)
- **Automatic reconnection**: Retries with configurable delay on connection loss
- **Structured logging**: JSON-style structured logs with configurable log levels
- **Zero dependencies beyond stdlib**: Only uses `goburrow/modbus` for the master client and `yaml.v3` for config

## Installation

```bash
go install github.com/nuhcevdet/modbus-tcp-mirror@latest
```

Or build from source:

```bash
git clone https://github.com/nuhcevdet/modbus-tcp-mirror.git
cd modbus-tcp-mirror
go build -o modbus-tcp-mirror .
```

## Quick Start

1. Copy and edit the config file:

```bash
cp config.yaml config.yaml
# Edit config.yaml with your source PLC address and register blocks
```

2. Run:

```bash
./modbus-tcp-mirror -config config.yaml
```

## Configuration

All settings are controlled via a single YAML file:

```yaml
# Source PLC (device to connect as Modbus Master)
source:
  ip: "192.168.1.10"
  port: 502
  slave_id: 1
  timeout_ms: 3000
  polling_interval_ms: 1000
  retry_count: 3
  retry_delay_ms: 500

# Modbus Slave Server (this application)
server:
  ip: "0.0.0.0"
  port: 5020
  slave_id: 1

# Register blocks to read from source
register_blocks:
  - name: "holding_registers_block1"
    function: "holding_register"
    start_address: 0
    count: 100

  - name: "input_registers_block1"
    function: "input_register"
    start_address: 0
    count: 50

  - name: "coils_block1"
    function: "coil"
    start_address: 0
    count: 64

  - name: "discrete_inputs_block1"
    function: "discrete_input"
    start_address: 0
    count: 64

# Logging
logging:
  level: "info"
  log_data_changes: true
```

### Configuration Reference

| Section | Field | Description | Default |
|---------|-------|-------------|---------|
| `source` | `ip` | Source device IP address | *required* |
| `source` | `port` | Source device Modbus TCP port | *required* |
| `source` | `slave_id` | Modbus slave/unit ID of source | `1` |
| `source` | `timeout_ms` | Read timeout in milliseconds | `3000` |
| `source` | `polling_interval_ms` | Polling interval in milliseconds | `1000` |
| `source` | `retry_count` | Number of reconnection attempts | `3` |
| `source` | `retry_delay_ms` | Delay between retries in milliseconds | `500` |
| `server` | `ip` | Listen IP for the slave server | `0.0.0.0` |
| `server` | `port` | Listen port for the slave server | `5020` |
| `server` | `slave_id` | Slave/unit ID to respond as | `1` |
| `register_blocks[]` | `name` | Human-readable block name (for logs) | *required* |
| `register_blocks[]` | `function` | `holding_register`, `input_register`, `coil`, or `discrete_input` | *required* |
| `register_blocks[]` | `start_address` | Start address of the block | *required* |
| `register_blocks[]` | `count` | Number of registers/coils to read | *required* |
| `logging` | `level` | Log level: `debug`, `info`, `warn`, `error` | `info` |

### Register Block Limits

Per the Modbus specification:
- **Holding/Input registers**: max 125 per block
- **Coils/Discrete inputs**: max 2000 per block

To read a larger range, split it into multiple blocks.

## Supported Modbus Functions

| Code | Function | Direction |
|------|----------|-----------|
| `0x01` | Read Coils | Read |
| `0x02` | Read Discrete Inputs | Read |
| `0x03` | Read Holding Registers | Read |
| `0x04` | Read Input Registers | Read |
| `0x05` | Write Single Coil | Write |
| `0x06` | Write Single Register | Write |
| `0x0F` | Write Multiple Coils | Write |
| `0x10` | Write Multiple Registers | Write |

## Architecture

```
main.go              Entry point, signal handling, orchestration
config/config.go     YAML config loading and validation
datastore/store.go   Thread-safe in-memory register/coil storage
poller/poller.go     Modbus master client, periodic polling
server/server.go     Modbus TCP slave server implementation
```

The **poller** reads from the source device and writes to the **datastore**. The **server** reads from the same datastore to respond to incoming Modbus requests. The datastore uses `sync.RWMutex` for safe concurrent access.

## Use Cases

- **PLC mirroring**: Create a read-only copy of a PLC for monitoring without adding load to the original device
- **Protocol bridging**: Expose a device on a different network/port
- **Testing & simulation**: Capture live PLC data and replay it for development
- **Data aggregation**: Combine multiple register blocks into a single accessible endpoint

## License

[MIT](LICENSE)
