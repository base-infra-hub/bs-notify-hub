package dispatch

import (
	"context"
	"encoding/json"
	"sync"

	"bs-notify-hub/pkg/pool"
)

type TargetType int8

const (
	TargetSingle    TargetType = iota // 1对1
	TargetMultiple                    // 1对多
	TargetBroadcast                   // 广播
)

// Message 核心调度消息体
type Message struct {
	TraceID    string          // 链路ID
	TenantID   string          // 租户ID
	TargetType TargetType      // 目标类型：单用户、多用户、广播
	TargetIDs  []string        // 目标用户ID列表（单用户时仅包含一个用户ID，广播时为空）
	Payload    json.RawMessage // 消息内容
}

var (
	global     *Dispatcher
	globalOnce sync.Once
)

func SetGlobal(d *Dispatcher) {
	globalOnce.Do(func() {
		global = d
	})
}

func GetGlobal() *Dispatcher {
	if global == nil {
		panic("dispatcher not initialized")
	}
	return global
}

type Dispatcher struct {
	broker   Broker
	connPool *pool.Manager
	runOnce  sync.Once
}

func NewDispatcher(broker Broker, pool *pool.Manager) *Dispatcher {
	return &Dispatcher{
		broker:   broker,
		connPool: pool,
	}
}

func (d *Dispatcher) Run() {
	d.runOnce.Do(func() {
		go d.run()
	})
}
func (d *Dispatcher) run() {
	for msg := range d.broker.Subscribe() {
		d.process(msg)
	}
}

// ====== 对外 API ======

func (d *Dispatcher) Join(group, userID, connID string, conn pool.Connection) {
	d.connPool.Register(group, userID, connID, conn)
}

func (d *Dispatcher) Leave(group, userID, connID string, conn pool.Connection) {
	d.connPool.Unregister(group, userID, connID, conn)
}

func (d *Dispatcher) TransmitMessage(_ context.Context, msg *Message) error {
	return d.broker.Publish(msg)
}

// ConnectionCount 返回当前在线连接总数
func (d *Dispatcher) ConnectionCount() int {
	if d.connPool == nil {
		return 0
	}
	return d.connPool.Count()
}

// OnlineGroupCount 返回在线租户数
func (d *Dispatcher) OnlineGroupCount() int {
	if d.connPool == nil {
		return 0
	}
	return d.connPool.GroupCount()
}

// OnlineUserCount 返回在线用户数
func (d *Dispatcher) OnlineUserCount() int {
	if d.connPool == nil {
		return 0
	}
	return d.connPool.UserCount()
}

// GroupStats 返回各租户在线统计
func (d *Dispatcher) GroupStats() map[string]map[string]interface{} {
	if d.connPool == nil {
		return nil
	}
	return d.connPool.GroupStats()
}

// GetOnlineUserIDs 返回指定租户下所有在线用户ID
func (d *Dispatcher) GetOnlineUserIDs(tenantID string) []string {
	if d.connPool == nil {
		return nil
	}
	return d.connPool.GetOnlineUserIDs(tenantID)
}

// ====== 核心分发逻辑======

func (d *Dispatcher) process(msg *Message) {
	data := msg.Payload
	switch msg.TargetType {
	case TargetSingle, TargetMultiple:
		for _, userID := range msg.TargetIDs {
			d.connPool.RangeUser(msg.TenantID, userID, func(connID string, conn pool.Connection) bool {
				_ = conn.Write(data)
				return true
			})
		}

	case TargetBroadcast:

		d.connPool.RangeGroup(msg.TenantID, func(uid, cid string, conn pool.Connection) bool {
			_ = conn.Write(data)
			return true
		})
	}
}
