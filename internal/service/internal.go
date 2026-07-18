package service

import (
	"context"
	"log"
	"runtime"
	"sync"
	"time"

	"bs-notify-hub/internal/dispatch"
	"bs-notify-hub/internal/model"
	"bs-notify-hub/pkg/db"
	"bs-notify-hub/pkg/redis"
)

// InternalService 提供内部看板统计与服务器健康状态
type InternalService struct {
	startTime  time.Time
	dispatcher *dispatch.Dispatcher
}

var (
	internalServiceInstance *InternalService
	internalServiceOnce     sync.Once
)

// GetInternalService 获取内部服务单例
func GetInternalService() *InternalService {
	internalServiceOnce.Do(func() {
		internalServiceInstance = &InternalService{
			startTime:  time.Now(),
			dispatcher: dispatch.GetGlobal(),
		}
	})
	return internalServiceInstance
}

// DashboardStats 看板统计数据
type DashboardStats struct {
	Uptime        string                `json:"uptime"`
	GoVersion     string                `json:"go_version"`
	Goroutines    int                   `json:"goroutines"`
	MemoryMB      float64               `json:"memory_mb"`
	DbHealthy     bool                  `json:"db_healthy"`
	RedisHealthy  bool                  `json:"redis_healthy"`
	TenantCount   int64                 `json:"tenant_count"`   // 历史租户总数
	UserCount     int64                 `json:"user_count"`     // 历史用户总数
	OnlineTenants int                   `json:"online_tenants"` // 当前在线租户数
	OnlineUsers   int                   `json:"online_users"`   // 当前在线用户数
	Connections   int                   `json:"connections"`    // 当前连接数
	GroupStats    map[string]*GroupStat `json:"group_stats"`    // 各租户在线详情
	RecentMsgs    []RecentMsg           `json:"recent_msgs"`    // 最近消息分发
}

// GroupStat 单个租户在线统计
type GroupStat struct {
	UserCount int `json:"user_count"`
	ConnCount int `json:"conn_count"`
}

// RecentMsg 最近分发消息
type RecentMsg struct {
	NotifyID   string    `json:"notify_id"`
	Title      string    `json:"title"`
	TenantID   string    `json:"tenant_id"`
	SenderType int8      `json:"sender_type"`
	TargetType int8      `json:"target_type"`
	CreatedAt  time.Time `json:"created_at"`
}

// Dashboard 获取实时看板数据
func (s *InternalService) Dashboard(ctx context.Context) DashboardStats {
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)

	tenantCount, userCount := s.countTenantsAndUsers(ctx)
	onlineTenants, onlineUsers, connections, groupStats := s.onlineStats()

	return DashboardStats{
		Uptime:        time.Since(s.startTime).Round(time.Second).String(),
		GoVersion:     runtime.Version(),
		Goroutines:    runtime.NumGoroutine(),
		MemoryMB:      roundMB(memStats.Sys),
		DbHealthy:     s.pingDB(ctx),
		RedisHealthy:  s.pingRedis(ctx),
		TenantCount:   tenantCount,
		UserCount:     userCount,
		OnlineTenants: onlineTenants,
		OnlineUsers:   onlineUsers,
		Connections:   connections,
		GroupStats:    groupStats,
		RecentMsgs:    s.recentMessages(ctx),
	}
}

func (s *InternalService) pingDB(ctx context.Context) bool {
	sqlDB, err := db.GetDB().DB()
	if err != nil {
		log.Printf("[Dashboard] 获取数据库连接失败: %v", err)
		return false
	}
	if err := sqlDB.PingContext(ctx); err != nil {
		log.Printf("[Dashboard] 数据库 Ping 失败: %v", err)
		return false
	}
	return true
}

func (s *InternalService) pingRedis(ctx context.Context) bool {
	if err := redis.GetClient().Ping(ctx).Err(); err != nil {
		log.Printf("[Dashboard] Redis Ping 失败: %v", err)
		return false
	}
	return true
}

func (s *InternalService) countTenantsAndUsers(ctx context.Context) (tenantCount, userCount int64) {
	database := db.GetDB().WithContext(ctx)

	// 历史租户总数
	if err := database.Model(&model.NotifyRecord{}).
		Select("COUNT(DISTINCT tenant_id)").
		Scan(&tenantCount).Error; err != nil {
		log.Printf("[Dashboard] 统计租户数失败: %v", err)
	}

	// 历史用户总数（已收到过通知的用户）
	if err := database.Model(&model.NotifyStatus{}).
		Select("COUNT(DISTINCT target_user_id)").
		Scan(&userCount).Error; err != nil {
		log.Printf("[Dashboard] 统计用户数失败: %v", err)
	}

	return
}

func (s *InternalService) onlineStats() (tenantCount, userCount, connectionCount int, groupStats map[string]*GroupStat) {
	if s.dispatcher == nil {
		return 0, 0, 0, nil
	}

	tenantCount = s.dispatcher.OnlineGroupCount()
	userCount = s.dispatcher.OnlineUserCount()
	connectionCount = s.dispatcher.ConnectionCount()

	rawStats := s.dispatcher.GroupStats()
	groupStats = make(map[string]*GroupStat, len(rawStats))
	for tenantID, stat := range rawStats {
		groupStats[tenantID] = &GroupStat{
			UserCount: stat["user_count"].(int),
			ConnCount: stat["conn_count"].(int),
		}
	}

	return
}

func (s *InternalService) recentMessages(ctx context.Context) []RecentMsg {
	var records []model.NotifyRecord
	if err := db.GetDB().WithContext(ctx).
		Order("created_at DESC").
		Limit(10).
		Find(&records).Error; err != nil {
		log.Printf("[Dashboard] 查询最近消息失败: %v", err)
		return nil
	}

	msgs := make([]RecentMsg, 0, len(records))
	for _, r := range records {
		msgs = append(msgs, RecentMsg{
			NotifyID:   r.NotifyID.String(),
			Title:      r.Title,
			TenantID:   r.TenantID,
			SenderType: r.SenderType,
			TargetType: r.TargetType,
			CreatedAt:  r.CreatedAt,
		})
	}
	return msgs
}

func roundMB(bytes uint64) float64 {
	mb := float64(bytes) / 1024 / 1024
	return float64(int(mb*100)) / 100
}
