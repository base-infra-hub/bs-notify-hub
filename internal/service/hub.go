package service

import (
	"bs-notify-hub/internal/dto"
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"sync"
	"time"

	"bs-notify-hub/internal/dispatch"

	"github.com/bytedance/sonic"
	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/common/hlog"
	"github.com/cloudwego/hertz/pkg/protocol/sse"
)

type NotifyEvent string

const (
	NotifyMsg NotifyEvent = "notify"
	Unread    NotifyEvent = "unread"
)

// HubSSEConnection 适配 SSE 连接，用于推送数据
type HubSSEConnection struct {
	MessageChan chan *dto.InternalMsg
	once        sync.Once
}

func (c *HubSSEConnection) Write(data interface{}) error {
	payload, ok := data.(json.RawMessage)
	if !ok {
		return fmt.Errorf("非法数据类型: 期待 json.RawMessage")
	}
	var msg dto.InternalMsg
	if err := sonic.Unmarshal(payload, &msg); err != nil {
		return fmt.Errorf("反序列化 InternalMsg 失败: %v", err)
	}
	select {
	case c.MessageChan <- &msg:
		return nil
	default:
		return fmt.Errorf("通知通道已满")
	}
}

func (c *HubSSEConnection) Close() error {
	c.once.Do(func() {
		close(c.MessageChan)
	})
	return nil
}

// HubService 订阅中心业务服务 (无状态)
type HubService struct{}

var (
	hubInstance *HubService
	hubOnce     sync.Once
)

// GetHubService 获取订阅中心单例
func GetHubService() *HubService {
	hubOnce.Do(func() {
		hubInstance = &HubService{}
	})
	return hubInstance
}
func (s *HubService) Subscribe(ctx context.Context, c *app.RequestContext, param *dto.SubscribeParam) {
	heartbeatTicker := time.NewTicker(1 * time.Second)
	conn := s.Register(param.TenantID, param.UserID, param.ConnID)
	hlog.Infof("[Notify-Hub] SSE 连接已建立 (Tenant:%s, User:%s, ConnID:%s)", param.TenantID, param.UserID, param.ConnID)
	defer func() {
		heartbeatTicker.Stop()
		s.Unregister(param.TenantID, param.UserID, param.ConnID, conn)
	}()
	w := sse.NewWriter(c)
	for {
		select {
		case <-ctx.Done():
			return
		case msg, ok := <-conn.MessageChan:
			if !ok {
				return
			}
			bytes, err := sonic.Marshal(msg.Data)
			if err != nil {
				hlog.Errorf("[Notify-Hub] 序列化失败 Type: %s, Error: %v", msg.Type, err)
				continue
			}
			if err := w.WriteEvent(param.ConnID, string(msg.Type), bytes); err != nil {
				hlog.Errorf("[Notify-Hub] 推送失败 ConnID: %s, Error: %v", param.ConnID, err)
				return
			}
		case t := <-heartbeatTicker.C:
			buf := []byte(`{"time":`)
			buf = strconv.AppendInt(buf, t.Unix(), 10)
			buf = append(buf, '}')
			if err := w.WriteEvent(param.ConnID, string(dto.Heartbeat), buf); err != nil {
				hlog.Errorf("[Notify-Hub] 心跳推送失败 ConnID: %s, Error: %v", param.ConnID, err)
				return
			}
		}
	}
}

// Register 创建连接并通过调度器加入连接池中
func (s *HubService) Register(tenantID, userID, connID string) *HubSSEConnection {
	conn := &HubSSEConnection{
		MessageChan: make(chan *dto.InternalMsg, 100),
	}
	dispatcher := dispatch.GetGlobal()
	dispatcher.Join(tenantID, userID, connID, conn)

	return conn
}

// Unregister 从全局调度器摘除连接
func (s *HubService) Unregister(tenantID, userID, connID string, exactConn *HubSSEConnection) {
	dispatcher := dispatch.GetGlobal()
	dispatcher.Leave(tenantID, userID, connID, exactConn)
}
