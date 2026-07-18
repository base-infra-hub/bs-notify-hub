package session

import (
	"crypto/rand"
	"encoding/hex"
	"sync"
	"time"
)

const sessionTTL = 2 * time.Hour

// entry 单个 Session 的存储条目
type entry struct {
	createdAt time.Time
}

// Store 内存 Session 存储（线程安全）
type Store struct {
	mu     sync.RWMutex
	data   map[string]entry
	stopGC chan struct{}
}

var globalStore = newStore()

func newStore() *Store {
	s := &Store{
		data:   make(map[string]entry),
		stopGC: make(chan struct{}),
	}
	go s.runGC()
	return s
}

// GetStore 获取全局 Session Store 单例
func GetStore() *Store {
	return globalStore
}

// Create 创建新 Session，返回 session ID
func (s *Store) Create() string {
	id := generateID()
	s.mu.Lock()
	s.data[id] = entry{createdAt: time.Now()}
	s.mu.Unlock()
	return id
}

// Valid 校验 session ID 是否有效（存在且未过期）
func (s *Store) Valid(id string) bool {
	if id == "" {
		return false
	}
	s.mu.RLock()
	e, ok := s.data[id]
	s.mu.RUnlock()
	if !ok {
		return false
	}
	return time.Since(e.createdAt) < sessionTTL
}

// Delete 主动删除 session（登出）
func (s *Store) Delete(id string) {
	s.mu.Lock()
	delete(s.data, id)
	s.mu.Unlock()
}

// runGC 每 10 分钟清理一次过期 session
func (s *Store) runGC() {
	ticker := time.NewTicker(10 * time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			s.sweep()
		case <-s.stopGC:
			return
		}
	}
}

func (s *Store) sweep() {
	now := time.Now()
	s.mu.Lock()
	for id, e := range s.data {
		if now.Sub(e.createdAt) >= sessionTTL {
			delete(s.data, id)
		}
	}
	s.mu.Unlock()
}

// generateID 生成 32 字节随机 hex session ID
func generateID() string {
	b := make([]byte, 32)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}
