package main

import (
	"sync"
	"time"
)

type value interface{}

type HashTtl struct {
	data  map[string]value
	times map[string]int64
	ttl   int64
	mutex *sync.RWMutex
}

func NewHashTtl(ttl int64) *HashTtl {
	return &HashTtl{data: make(map[string]value),
		times: make(map[string]int64),
		ttl:   ttl,
		mutex: &sync.RWMutex{}}
}

func (h *HashTtl) get(key string) (value, bool) {
	h.mutex.RLock()
	defer h.mutex.RUnlock()

	entry_time, present := h.times[key]
	if !present {
		return nil, false
	}
	cur_time := time.Now().Unix()
	if cur_time-entry_time > h.ttl {
		delete(h.data, key)
		delete(h.times, key)
		return nil, false
	}
	return h.data[key], true
}

func (h *HashTtl) set(key string, val value) {
	h.mutex.Lock()
	defer h.mutex.Unlock()

	h.data[key] = val
	h.times[key] = time.Now().Unix()
}
