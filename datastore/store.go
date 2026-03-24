package datastore

import "sync"

// Store holds Modbus register/coil data in a thread-safe manner.
// The poller writes data read from the source device; the slave server reads from here.
type Store struct {
	mu               sync.RWMutex
	holdingRegisters map[uint16]uint16
	inputRegisters   map[uint16]uint16
	coils            map[uint16]bool
	discreteInputs   map[uint16]bool
}

func New() *Store {
	return &Store{
		holdingRegisters: make(map[uint16]uint16),
		inputRegisters:   make(map[uint16]uint16),
		coils:            make(map[uint16]bool),
		discreteInputs:   make(map[uint16]bool),
	}
}

func (s *Store) SetHoldingRegisters(start uint16, values []uint16) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i, v := range values {
		s.holdingRegisters[start+uint16(i)] = v
	}
}

func (s *Store) GetHoldingRegister(addr uint16) uint16 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.holdingRegisters[addr]
}

func (s *Store) GetHoldingRegisters(start, count uint16) []uint16 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]uint16, count)
	for i := uint16(0); i < count; i++ {
		result[i] = s.holdingRegisters[start+i]
	}
	return result
}

func (s *Store) SetInputRegisters(start uint16, values []uint16) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i, v := range values {
		s.inputRegisters[start+uint16(i)] = v
	}
}

func (s *Store) GetInputRegisters(start, count uint16) []uint16 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]uint16, count)
	for i := uint16(0); i < count; i++ {
		result[i] = s.inputRegisters[start+i]
	}
	return result
}

func (s *Store) SetCoils(start uint16, values []bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i, v := range values {
		s.coils[start+uint16(i)] = v
	}
}

func (s *Store) GetCoils(start, count uint16) []bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]bool, count)
	for i := uint16(0); i < count; i++ {
		result[i] = s.coils[start+i]
	}
	return result
}

func (s *Store) SetDiscreteInputs(start uint16, values []bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i, v := range values {
		s.discreteInputs[start+uint16(i)] = v
	}
}

func (s *Store) GetDiscreteInputs(start, count uint16) []bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]bool, count)
	for i := uint16(0); i < count; i++ {
		result[i] = s.discreteInputs[start+i]
	}
	return result
}
