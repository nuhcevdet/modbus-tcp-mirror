package poller

import (
	"encoding/binary"
	"fmt"
	"log/slog"
	"net"
	"sync"
	"time"

	"modbusslave-test/config"
	"modbusslave-test/datastore"

	"github.com/goburrow/modbus"
)

type Poller struct {
	cfg     *config.AppConfig
	store   *datastore.Store
	client  modbus.Client
	handler *modbus.TCPClientHandler
	stopCh  chan struct{}
	wg      sync.WaitGroup
	logger  *slog.Logger
}

func New(cfg *config.AppConfig, store *datastore.Store, logger *slog.Logger) *Poller {
	return &Poller{
		cfg:    cfg,
		store:  store,
		stopCh: make(chan struct{}),
		logger: logger,
	}
}

func (p *Poller) Start() error {
	if err := p.connect(); err != nil {
		return err
	}

	p.wg.Add(1)
	go p.pollLoop()

	p.logger.Info("poller started",
		"source", p.cfg.SourceAddress(),
		"polling_interval_ms", p.cfg.Source.PollingInterval,
		"block_count", len(p.cfg.RegisterBlocks),
	)
	return nil
}

func (p *Poller) Stop() {
	close(p.stopCh)
	p.wg.Wait()
	if p.handler != nil {
		p.handler.Close()
	}
	p.logger.Info("poller stopped")
}

func (p *Poller) connect() error {
	addr := p.cfg.SourceAddress()

	handler := modbus.NewTCPClientHandler(addr)
	handler.Timeout = time.Duration(p.cfg.Source.TimeoutMs) * time.Millisecond
	handler.SlaveId = p.cfg.Source.SlaveID
	handler.IdleTimeout = 10 * time.Second

	if err := handler.Connect(); err != nil {
		return fmt.Errorf("modbus connection failed (%s): %w", addr, err)
	}

	p.handler = handler
	p.client = modbus.NewClient(handler)
	p.logger.Info("connected to source PLC", "address", addr, "slave_id", p.cfg.Source.SlaveID)
	return nil
}

func (p *Poller) reconnect() error {
	if p.handler != nil {
		p.handler.Close()
	}

	for attempt := 1; attempt <= p.cfg.Source.RetryCount; attempt++ {
		p.logger.Warn("reconnection attempt", "attempt", attempt, "max", p.cfg.Source.RetryCount)

		if err := p.connect(); err != nil {
			p.logger.Error("connection failed", "attempt", attempt, "error", err)
			time.Sleep(time.Duration(p.cfg.Source.RetryDelayMs) * time.Millisecond)
			continue
		}
		return nil
	}
	return fmt.Errorf("all reconnection attempts failed")
}

func (p *Poller) pollLoop() {
	defer p.wg.Done()
	ticker := time.NewTicker(time.Duration(p.cfg.Source.PollingInterval) * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-p.stopCh:
			return
		case <-ticker.C:
			if err := p.pollAll(); err != nil {
				p.logger.Error("poll error, reconnecting", "error", err)
				if reconnErr := p.reconnect(); reconnErr != nil {
					p.logger.Error("reconnection failed", "error", reconnErr)
				}
			}
		}
	}
}

func (p *Poller) pollAll() error {
	for _, block := range p.cfg.RegisterBlocks {
		if err := p.pollBlock(block); err != nil {
			return err
		}
	}
	return nil
}

func (p *Poller) pollBlock(block config.RegisterBlock) error {
	ft := block.FunctionType()

	switch ft {
	case "holding_register":
		return p.readHoldingRegisters(block)
	case "input_register":
		return p.readInputRegisters(block)
	case "coil":
		return p.readCoils(block)
	case "discrete_input":
		return p.readDiscreteInputs(block)
	default:
		return fmt.Errorf("unknown function type: %s", ft)
	}
}

func (p *Poller) readHoldingRegisters(block config.RegisterBlock) error {
	results, err := p.client.ReadHoldingRegisters(block.StartAddress, block.Count)
	if err != nil {
		if isConnectionError(err) {
			return fmt.Errorf("holding register read connection error: %w", err)
		}
		p.logger.Error("holding register read error",
			"block", block.Name, "start", block.StartAddress, "count", block.Count, "error", err)
		return nil
	}

	values := bytesToUint16(results)
	p.store.SetHoldingRegisters(block.StartAddress, values)

	p.logger.Debug("holding registers read",
		"block", block.Name, "start", block.StartAddress, "count", len(values))
	return nil
}

func (p *Poller) readInputRegisters(block config.RegisterBlock) error {
	results, err := p.client.ReadInputRegisters(block.StartAddress, block.Count)
	if err != nil {
		if isConnectionError(err) {
			return fmt.Errorf("input register read connection error: %w", err)
		}
		p.logger.Error("input register read error",
			"block", block.Name, "start", block.StartAddress, "count", block.Count, "error", err)
		return nil
	}

	values := bytesToUint16(results)
	p.store.SetInputRegisters(block.StartAddress, values)

	p.logger.Debug("input registers read",
		"block", block.Name, "start", block.StartAddress, "count", len(values))
	return nil
}

func (p *Poller) readCoils(block config.RegisterBlock) error {
	results, err := p.client.ReadCoils(block.StartAddress, block.Count)
	if err != nil {
		if isConnectionError(err) {
			return fmt.Errorf("coil read connection error: %w", err)
		}
		p.logger.Error("coil read error",
			"block", block.Name, "start", block.StartAddress, "count", block.Count, "error", err)
		return nil
	}

	values := bytesToBools(results, block.Count)
	p.store.SetCoils(block.StartAddress, values)

	p.logger.Debug("coils read",
		"block", block.Name, "start", block.StartAddress, "count", len(values))
	return nil
}

func (p *Poller) readDiscreteInputs(block config.RegisterBlock) error {
	results, err := p.client.ReadDiscreteInputs(block.StartAddress, block.Count)
	if err != nil {
		if isConnectionError(err) {
			return fmt.Errorf("discrete input read connection error: %w", err)
		}
		p.logger.Error("discrete input read error",
			"block", block.Name, "start", block.StartAddress, "count", block.Count, "error", err)
		return nil
	}

	values := bytesToBools(results, block.Count)
	p.store.SetDiscreteInputs(block.StartAddress, values)

	p.logger.Debug("discrete inputs read",
		"block", block.Name, "start", block.StartAddress, "count", len(values))
	return nil
}

func bytesToUint16(data []byte) []uint16 {
	count := len(data) / 2
	values := make([]uint16, count)
	for i := 0; i < count; i++ {
		values[i] = binary.BigEndian.Uint16(data[i*2 : i*2+2])
	}
	return values
}

func bytesToBools(data []byte, count uint16) []bool {
	values := make([]bool, count)
	for i := uint16(0); i < count; i++ {
		byteIdx := i / 8
		bitIdx := i % 8
		if int(byteIdx) < len(data) {
			values[i] = (data[byteIdx]>>bitIdx)&1 == 1
		}
	}
	return values
}

func isConnectionError(err error) bool {
	if _, ok := err.(*net.OpError); ok {
		return true
	}
	return false
}
