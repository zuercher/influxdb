package identity

import "sync"

type Map struct {
	mu sync.RWMutex

	lookup map[string]uint64
	items  [][]byte
}

func NewMap() *Map {
	return &Map{
		lookup: make(map[string]uint64, 1024),
		items:  make([][]byte, 0, 1024),
	}
}

func (m *Map) Set(s []byte) (uint64, bool) {
	id, ok := m.Lookup(s)
	if ok {
		return id, false
	}

	m.mu.Lock()
	if id, ok := m.lookup[string(s)]; ok {
		m.mu.Unlock()
		return id, false
	}

	m.items = append(m.items, s)
	id = uint64(len(m.items) - 1)
	m.lookup[string(s)] = id
	m.mu.Unlock()
	return id, true
}

func (m *Map) Lookup(s []byte) (uint64, bool) {
	m.mu.RLock()
	id, ok := m.lookup[string(s)]
	m.mu.RUnlock()

	return id, ok
}

func (m *Map) Get(id uint64) []byte {
	m.mu.RLock()

	if id < 0 || id >= uint64(len(m.items)) {
		m.mu.RUnlock()
		return nil
	}

	s := m.items[id]
	m.mu.RUnlock()
	return s
}

func (m *Map) Remove(s []byte) (uint64, bool) {
	m.mu.Lock()
	id, ok := m.lookup[string(s)]
	if ok {
		delete(m.lookup, string(s))
		m.items[id] = nil
	}
	m.mu.Unlock()
	return id, ok
}
