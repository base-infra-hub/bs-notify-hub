package pool

import (
	"log"
	"sync"
)

// Manager 全局连接池管理器
type Manager struct {
	groups   map[string]*Group
	globalMu sync.RWMutex
}

// NewManager 创建 manager
func NewManager() *Manager {
	return &Manager{
		groups: make(map[string]*Group),
	}
}

// getOrCreateGroup 获取或创建 group
func (m *Manager) getOrCreateGroup(name string) *Group {
	m.globalMu.Lock()
	defer m.globalMu.Unlock()

	if g, ok := m.groups[name]; ok {
		return g
	}

	g := &Group{
		users: make(map[string]map[string]Connection),
	}

	m.groups[name] = g
	return g
}

// getGroup 只读获取 group
func (m *Manager) getGroup(name string) *Group {
	m.globalMu.RLock()
	defer m.globalMu.RUnlock()

	return m.groups[name]
}

// Register 注册连接
func (m *Manager) Register(groupName, userID, connID string, conn Connection) {
	g := m.getOrCreateGroup(groupName)
	g.Register(userID, connID, conn)
}

// Unregister 注销连接 + 自动回收 group
func (m *Manager) Unregister(groupName, userID, connID string, exactConn Connection) {
	g := m.getGroup(groupName)
	if g == nil {
		return
	}

	empty := g.Unregister(userID, connID, exactConn)

	if !empty {
		return
	}

	m.globalMu.Lock()
	defer m.globalMu.Unlock()

	if current, ok := m.groups[groupName]; ok {
		if current.IsEmpty() {
			delete(m.groups, groupName)
			log.Printf("[Pool] 回收 Group:%s", groupName)
		}
	}
}

// RangeUser 遍历某用户所有连接
func (m *Manager) RangeUser(groupName, userID string, fn func(connID string, conn Connection) bool) {
	g := m.getGroup(groupName)
	if g == nil {
		return
	}
	g.RangeUser(userID, fn)
}

// RangeGroup 遍历整个 group
func (m *Manager) RangeGroup(groupName string, fn func(userID, connID string, conn Connection) bool) {
	g := m.getGroup(groupName)
	if g == nil {
		return
	}

	g.mu.RLock()
	defer g.mu.RUnlock()

	for uid, userClients := range g.users {
		for cid, conn := range userClients {
			if !fn(uid, cid, conn) {
				return
			}
		}
	}
}

// Count 统计连接数
func (m *Manager) Count() int {
	m.globalMu.RLock()
	defer m.globalMu.RUnlock()

	total := 0
	for _, g := range m.groups {
		g.mu.RLock()
		for _, userClients := range g.users {
			total += len(userClients)
		}
		g.mu.RUnlock()
	}
	return total
}

// GroupCount 统计组数量（在线租户数）
func (m *Manager) GroupCount() int {
	m.globalMu.RLock()
	defer m.globalMu.RUnlock()

	return len(m.groups)
}

// UserCount 统计在线用户数（去重，跨所有 group 统计）
func (m *Manager) UserCount() int {
	m.globalMu.RLock()
	defer m.globalMu.RUnlock()

	userSet := make(map[string]struct{})
	for _, g := range m.groups {
		g.mu.RLock()
		for userID := range g.users {
			userSet[userID] = struct{}{}
		}
		g.mu.RUnlock()
	}
	return len(userSet)
}

// GetOnlineUserIDs 返回指定 group 下所有在线用户ID（去重）
func (m *Manager) GetOnlineUserIDs(groupName string) []string {
	m.globalMu.RLock()
	defer m.globalMu.RUnlock()

	g := m.groups[groupName]
	if g == nil {
		return nil
	}

	g.mu.RLock()
	defer g.mu.RUnlock()

	users := make([]string, 0, len(g.users))
	for userID := range g.users {
		users = append(users, userID)
	}
	return users
}

// GroupStats 返回每个组的在线用户数与连接数
func (m *Manager) GroupStats() map[string]map[string]interface{} {
	m.globalMu.RLock()
	defer m.globalMu.RUnlock()

	stats := make(map[string]map[string]interface{})
	for name, g := range m.groups {
		g.mu.RLock()
		userCount := len(g.users)
		connCount := 0
		for _, userClients := range g.users {
			connCount += len(userClients)
		}
		g.mu.RUnlock()

		stats[name] = map[string]interface{}{
			"user_count": userCount,
			"conn_count": connCount,
		}
	}
	return stats
}
