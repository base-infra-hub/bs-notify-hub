package pool

import "sync"

// Group 代表一个租户或一个房间
type Group struct {
	mu    sync.RWMutex
	users map[string]map[string]Connection
}

// Register 注册连接（单用户多端）
func (g *Group) Register(userID, connID string, conn Connection) {
	g.mu.Lock()
	defer g.mu.Unlock()

	if g.users == nil {
		g.users = make(map[string]map[string]Connection)
	}

	if _, ok := g.users[userID]; !ok {
		g.users[userID] = make(map[string]Connection)
	}

	if old, exists := g.users[userID][connID]; exists {
		_ = old.Close()
	}

	g.users[userID][connID] = conn
}

// Unregister 删除连接，并返回 group 是否已空
func (g *Group) Unregister(userID, connID string, exactConn Connection) bool {
	g.mu.Lock()
	defer g.mu.Unlock()

	userClients, ok := g.users[userID]
	if !ok {
		return len(g.users) == 0
	}

	conn, exists := userClients[connID]
	if !exists {
		return len(g.users) == 0
	}

	if exactConn != nil && conn != exactConn {
		return len(g.users) == 0
	}

	_ = conn.Close()
	delete(userClients, connID)

	if len(userClients) == 0 {
		delete(g.users, userID)
	}

	return len(g.users) == 0
}

// RangeUser 遍历用户下所有连接
func (g *Group) RangeUser(userID string, fn func(connID string, conn Connection) bool) {
	g.mu.RLock()
	defer g.mu.RUnlock()

	if userClients, ok := g.users[userID]; ok {
		for cid, conn := range userClients {
			if !fn(cid, conn) {
				return
			}
		}
	}
}

// IsEmpty 判断 group 是否为空
func (g *Group) IsEmpty() bool {
	g.mu.RLock()
	defer g.mu.RUnlock()

	return len(g.users) == 0
}
