# modbus-tcp-mirror
A lightweight Go application that creates an exact replica of a Modbus TCP device. It connects to a source PLC/device as a Modbus master, periodically polls configured register blocks, and re-exposes the same data as a Modbus TCP slave server.
