package session

import (
	"crypto/rand"
	"encoding/hex"
	"sync"
	"time"
)

// defaultSessionTTLSeconds 默认会话有效期（2 小时），未调用 InitTTL 时生效
const defaultSessionTTLSeconds = 2 * 60 * 60

// Session 单个会话的存储信息
type Session struct {
	// TenantID 会话绑定的租户 ID，管理员控制台会话固定为空串
	TenantID  string
	createdAt time.Time
}

// Store 内存 Session 存储（线程安全）
type Store struct {
	mu     sync.RWMutex
	data   map[string]Session
	ttl    time.Duration
	stopGC chan struct{}
}

var globalStore = newStore()

func newStore() *Store {
	s := &Store{
		data:   make(map[string]Session),
		ttl:    defaultSessionTTLSeconds * time.Second,
		stopGC: make(chan struct{}),
	}
	go s.runGC()
	return s
}

// GetStore 获取全局 Session Store 单例
func GetStore() *Store {
	return globalStore
}

// InitTTL 初始化 Session 有效期（秒），需在配置加载后、对外服务前调用
// ttlSeconds <= 0 时保持默认值 2 小时
func InitTTL(ttlSeconds int) {
	if ttlSeconds <= 0 {
		return
	}
	globalStore.mu.Lock()
	globalStore.ttl = time.Duration(ttlSeconds) * time.Second
	globalStore.mu.Unlock()
}

// TTLSeconds 返回当前生效的 Session 有效期（秒），用于设置 Cookie MaxAge
func TTLSeconds() int {
	globalStore.mu.RLock()
	defer globalStore.mu.RUnlock()
	return int(globalStore.ttl / time.Second)
}

// Create 创建新 Session，返回 session ID
// tenantID 传空串表示管理员控制台会话
func (s *Store) Create(tenantID string) string {
	id := generateID()
	s.mu.Lock()
	s.data[id] = Session{TenantID: tenantID, createdAt: time.Now()}
	s.mu.Unlock()
	return id
}

// Get 查询会话，返回会话信息；bool 表示会话存在且未过期
func (s *Store) Get(id string) (Session, bool) {
	if id == "" {
		return Session{}, false
	}
	s.mu.RLock()
	e, ok := s.data[id]
	ttl := s.ttl
	s.mu.RUnlock()
	if !ok {
		return Session{}, false
	}
	if time.Since(e.createdAt) >= ttl {
		return Session{}, false
	}
	return e, true
}

// Valid 校验 session ID 是否有效（存在且未过期）
func (s *Store) Valid(id string) bool {
	_, ok := s.Get(id)
	return ok
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
		if now.Sub(e.createdAt) >= s.ttl {
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
