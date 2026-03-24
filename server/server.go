package server

import (
	"encoding/binary"
	"fmt"
	"io"
	"log/slog"
	"net"
	"sync"

	"modbusslave-test/config"
	"modbusslave-test/datastore"
)

const (
	mbapHeaderSize = 7 // Transaction ID (2) + Protocol ID (2) + Length (2) + Unit ID (1)

	fcReadCoils              = 0x01
	fcReadDiscreteInputs     = 0x02
	fcReadHoldingRegisters   = 0x03
	fcReadInputRegisters     = 0x04
	fcWriteSingleCoil        = 0x05
	fcWriteSingleRegister    = 0x06
	fcWriteMultipleCoils     = 0x0F
	fcWriteMultipleRegisters = 0x10

	exIllegalFunction    = 0x01
	exIllegalDataAddress = 0x02
	exSlaveDeviceFailure = 0x04
)

type Server struct {
	cfg      *config.AppConfig
	store    *datastore.Store
	listener net.Listener
	stopCh   chan struct{}
	wg       sync.WaitGroup
	logger   *slog.Logger
}

func New(cfg *config.AppConfig, store *datastore.Store, logger *slog.Logger) *Server {
	return &Server{
		cfg:    cfg,
		store:  store,
		stopCh: make(chan struct{}),
		logger: logger,
	}
}

func (s *Server) Start() error {
	addr := s.cfg.ServerAddress()
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("failed to start TCP listener (%s): %w", addr, err)
	}
	s.listener = listener

	s.wg.Add(1)
	go s.acceptLoop()

	s.logger.Info("modbus slave server started", "address", addr, "slave_id", s.cfg.Server.SlaveID)
	return nil
}

func (s *Server) Stop() {
	close(s.stopCh)
	if s.listener != nil {
		s.listener.Close()
	}
	s.wg.Wait()
	s.logger.Info("modbus slave server stopped")
}

func (s *Server) acceptLoop() {
	defer s.wg.Done()
	for {
		conn, err := s.listener.Accept()
		if err != nil {
			select {
			case <-s.stopCh:
				return
			default:
				s.logger.Error("accept error", "error", err)
				continue
			}
		}
		s.logger.Info("new client connected", "remote", conn.RemoteAddr().String())
		s.wg.Add(1)
		go s.handleConnection(conn)
	}
}

func (s *Server) handleConnection(conn net.Conn) {
	defer s.wg.Done()
	defer conn.Close()

	buf := make([]byte, 512)

	for {
		select {
		case <-s.stopCh:
			return
		default:
		}

		n, err := conn.Read(buf)
		if err != nil {
			if err != io.EOF {
				s.logger.Debug("client read error", "error", err)
			}
			s.logger.Info("client disconnected", "remote", conn.RemoteAddr().String())
			return
		}

		if n < mbapHeaderSize+1 {
			continue
		}

		response := s.processRequest(buf[:n])
		if response != nil {
			if _, err := conn.Write(response); err != nil {
				s.logger.Error("response write error", "error", err)
				return
			}
		}
	}
}

func (s *Server) processRequest(request []byte) []byte {
	if len(request) < mbapHeaderSize+1 {
		return nil
	}

	transactionID := binary.BigEndian.Uint16(request[0:2])
	protocolID := binary.BigEndian.Uint16(request[2:4])
	unitID := request[6]
	functionCode := request[7]

	if protocolID != 0 {
		return nil
	}

	_ = unitID

	var pdu []byte
	var exceptionCode byte

	switch functionCode {
	case fcReadHoldingRegisters:
		pdu, exceptionCode = s.handleReadHoldingRegisters(request[7:])
	case fcReadInputRegisters:
		pdu, exceptionCode = s.handleReadInputRegisters(request[7:])
	case fcReadCoils:
		pdu, exceptionCode = s.handleReadCoils(request[7:])
	case fcReadDiscreteInputs:
		pdu, exceptionCode = s.handleReadDiscreteInputs(request[7:])
	case fcWriteSingleCoil:
		pdu, exceptionCode = s.handleWriteSingleCoil(request[7:])
	case fcWriteSingleRegister:
		pdu, exceptionCode = s.handleWriteSingleRegister(request[7:])
	case fcWriteMultipleCoils:
		pdu, exceptionCode = s.handleWriteMultipleCoils(request[7:])
	case fcWriteMultipleRegisters:
		pdu, exceptionCode = s.handleWriteMultipleRegisters(request[7:])
	default:
		return s.buildExceptionResponse(transactionID, unitID, functionCode, exIllegalFunction)
	}

	if exceptionCode != 0 {
		return s.buildExceptionResponse(transactionID, unitID, functionCode, exceptionCode)
	}

	return s.buildResponse(transactionID, unitID, pdu)
}

func (s *Server) handleReadHoldingRegisters(pdu []byte) ([]byte, byte) {
	if len(pdu) < 5 {
		return nil, exIllegalDataAddress
	}
	startAddr := binary.BigEndian.Uint16(pdu[1:3])
	quantity := binary.BigEndian.Uint16(pdu[3:5])

	if quantity == 0 || quantity > 125 {
		return nil, exIllegalDataAddress
	}

	values := s.store.GetHoldingRegisters(startAddr, quantity)

	byteCount := byte(quantity * 2)
	resp := make([]byte, 2+int(byteCount))
	resp[0] = fcReadHoldingRegisters
	resp[1] = byteCount
	for i, v := range values {
		binary.BigEndian.PutUint16(resp[2+i*2:], v)
	}
	return resp, 0
}

func (s *Server) handleReadInputRegisters(pdu []byte) ([]byte, byte) {
	if len(pdu) < 5 {
		return nil, exIllegalDataAddress
	}
	startAddr := binary.BigEndian.Uint16(pdu[1:3])
	quantity := binary.BigEndian.Uint16(pdu[3:5])

	if quantity == 0 || quantity > 125 {
		return nil, exIllegalDataAddress
	}

	values := s.store.GetInputRegisters(startAddr, quantity)

	byteCount := byte(quantity * 2)
	resp := make([]byte, 2+int(byteCount))
	resp[0] = fcReadInputRegisters
	resp[1] = byteCount
	for i, v := range values {
		binary.BigEndian.PutUint16(resp[2+i*2:], v)
	}
	return resp, 0
}

func (s *Server) handleReadCoils(pdu []byte) ([]byte, byte) {
	if len(pdu) < 5 {
		return nil, exIllegalDataAddress
	}
	startAddr := binary.BigEndian.Uint16(pdu[1:3])
	quantity := binary.BigEndian.Uint16(pdu[3:5])

	if quantity == 0 || quantity > 2000 {
		return nil, exIllegalDataAddress
	}

	values := s.store.GetCoils(startAddr, quantity)

	byteCount := byte((quantity + 7) / 8)
	resp := make([]byte, 2+int(byteCount))
	resp[0] = fcReadCoils
	resp[1] = byteCount
	for i, v := range values {
		if v {
			byteIdx := i / 8
			bitIdx := uint(i % 8)
			resp[2+byteIdx] |= 1 << bitIdx
		}
	}
	return resp, 0
}

func (s *Server) handleReadDiscreteInputs(pdu []byte) ([]byte, byte) {
	if len(pdu) < 5 {
		return nil, exIllegalDataAddress
	}
	startAddr := binary.BigEndian.Uint16(pdu[1:3])
	quantity := binary.BigEndian.Uint16(pdu[3:5])

	if quantity == 0 || quantity > 2000 {
		return nil, exIllegalDataAddress
	}

	values := s.store.GetDiscreteInputs(startAddr, quantity)

	byteCount := byte((quantity + 7) / 8)
	resp := make([]byte, 2+int(byteCount))
	resp[0] = fcReadDiscreteInputs
	resp[1] = byteCount
	for i, v := range values {
		if v {
			byteIdx := i / 8
			bitIdx := uint(i % 8)
			resp[2+byteIdx] |= 1 << bitIdx
		}
	}
	return resp, 0
}

func (s *Server) handleWriteSingleCoil(pdu []byte) ([]byte, byte) {
	if len(pdu) < 5 {
		return nil, exIllegalDataAddress
	}
	addr := binary.BigEndian.Uint16(pdu[1:3])
	value := binary.BigEndian.Uint16(pdu[3:5])

	var boolVal bool
	if value == 0xFF00 {
		boolVal = true
	} else if value != 0x0000 {
		return nil, exIllegalDataAddress
	}

	s.store.SetCoils(addr, []bool{boolVal})
	s.logger.Debug("single coil written", "addr", addr, "value", boolVal)

	resp := make([]byte, 5)
	copy(resp, pdu[:5])
	return resp, 0
}

func (s *Server) handleWriteSingleRegister(pdu []byte) ([]byte, byte) {
	if len(pdu) < 5 {
		return nil, exIllegalDataAddress
	}
	addr := binary.BigEndian.Uint16(pdu[1:3])
	value := binary.BigEndian.Uint16(pdu[3:5])

	s.store.SetHoldingRegisters(addr, []uint16{value})
	s.logger.Debug("single register written", "addr", addr, "value", value)

	resp := make([]byte, 5)
	copy(resp, pdu[:5])
	return resp, 0
}

func (s *Server) handleWriteMultipleCoils(pdu []byte) ([]byte, byte) {
	if len(pdu) < 6 {
		return nil, exIllegalDataAddress
	}
	startAddr := binary.BigEndian.Uint16(pdu[1:3])
	quantity := binary.BigEndian.Uint16(pdu[3:5])
	byteCount := pdu[5]

	if len(pdu) < 6+int(byteCount) {
		return nil, exIllegalDataAddress
	}

	coilData := pdu[6 : 6+byteCount]
	values := make([]bool, quantity)
	for i := uint16(0); i < quantity; i++ {
		byteIdx := i / 8
		bitIdx := i % 8
		values[i] = (coilData[byteIdx]>>bitIdx)&1 == 1
	}

	s.store.SetCoils(startAddr, values)
	s.logger.Debug("multiple coils written", "start", startAddr, "count", quantity)

	resp := make([]byte, 5)
	resp[0] = fcWriteMultipleCoils
	binary.BigEndian.PutUint16(resp[1:3], startAddr)
	binary.BigEndian.PutUint16(resp[3:5], quantity)
	return resp, 0
}

func (s *Server) handleWriteMultipleRegisters(pdu []byte) ([]byte, byte) {
	if len(pdu) < 6 {
		return nil, exIllegalDataAddress
	}
	startAddr := binary.BigEndian.Uint16(pdu[1:3])
	quantity := binary.BigEndian.Uint16(pdu[3:5])
	byteCount := pdu[5]

	if len(pdu) < 6+int(byteCount) {
		return nil, exIllegalDataAddress
	}

	regData := pdu[6 : 6+byteCount]
	values := make([]uint16, quantity)
	for i := uint16(0); i < quantity; i++ {
		values[i] = binary.BigEndian.Uint16(regData[i*2 : i*2+2])
	}

	s.store.SetHoldingRegisters(startAddr, values)
	s.logger.Debug("multiple registers written", "start", startAddr, "count", quantity)

	resp := make([]byte, 5)
	resp[0] = fcWriteMultipleRegisters
	binary.BigEndian.PutUint16(resp[1:3], startAddr)
	binary.BigEndian.PutUint16(resp[3:5], quantity)
	return resp, 0
}

func (s *Server) buildResponse(transactionID uint16, unitID byte, pdu []byte) []byte {
	length := uint16(len(pdu) + 1) // PDU + Unit ID
	resp := make([]byte, mbapHeaderSize+len(pdu))
	binary.BigEndian.PutUint16(resp[0:2], transactionID)
	binary.BigEndian.PutUint16(resp[2:4], 0) // Protocol ID
	binary.BigEndian.PutUint16(resp[4:6], length)
	resp[6] = unitID
	copy(resp[7:], pdu)
	return resp
}

func (s *Server) buildExceptionResponse(transactionID uint16, unitID byte, functionCode byte, exceptionCode byte) []byte {
	pdu := []byte{functionCode | 0x80, exceptionCode}
	return s.buildResponse(transactionID, unitID, pdu)
}
