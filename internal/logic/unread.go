package logic

import (
	"bs-notify-hub/internal/constant"
	"bs-notify-hub/internal/repository"
	"bs-notify-hub/pkg/db"
	"bs-notify-hub/pkg/redis"
	"context"
	"fmt"
	"sync"

	"github.com/cloudwego/hertz/pkg/common/hlog"
	"golang.org/x/sync/singleflight"
)

type NotifyUnreadService struct {
	cacheRepo *repository.NotifyUnreadCacheRepo
	dbRepo    *repository.NotifyUnreadDBRepo
	sfGroup   singleflight.Group
}

var (
	unreadServiceInstance *NotifyUnreadService
	unreadServiceOnce     sync.Once
)

func GetNotifyUnreadService() *NotifyUnreadService {
	unreadServiceOnce.Do(func() {
		unreadServiceInstance = &NotifyUnreadService{
			cacheRepo: repository.NewNotifyUnreadCacheRepo(redis.GetClient()),
			dbRepo:    repository.NewNotifyUnreadDBRepo(db.GetDB()),
			sfGroup:   singleflight.Group{},
		}
	})
	return unreadServiceInstance
}

// ---  走数据库全量同步相关缓存值 ---

// ReconcilePersonalUnread 同步个人私信未读数
func (s *NotifyUnreadService) ReconcilePersonalUnread(ctx context.Context, tenantID, userID string) error {
	sfKey := fmt.Sprintf("rec:user:%s:%s", tenantID, userID)
	_, err, _ := s.sfGroup.Do(sfKey, func() (interface{}, error) {
		unreadCount, err := s.dbRepo.CountUnreadUser(tenantID, userID)
		if err != nil {
			hlog.Errorf("[GetNotifyUnreadService] 从数据库: %v", err)
			return nil, err
		}
		return nil, s.cacheRepo.Set(ctx, constant.GetUnreadPersonalKey(tenantID, userID), unreadCount)
	})
	return err
}

// ReconcileTenantUnread 同步租户类的非广播未读数和 Op UserID 池
func (s *NotifyUnreadService) ReconcileTenantUnread(ctx context.Context, tenantID, userID string) error {
	sfKey := fmt.Sprintf("rec:tenant:%s:%s", tenantID, userID)

	_, err, _ := s.sfGroup.Do(sfKey, func() (interface{}, error) {
		opIDs, err := s.dbRepo.GetBroadcastOpIDs(tenantID, userID)
		if err != nil {
			return nil, err
		}
		tenantUnread, err := s.dbRepo.CountUnreadTenant(tenantID, userID)
		if err != nil {
			return nil, err
		}
		err = s.cacheRepo.Set(ctx, constant.GetUnreadTenantKey(tenantID, userID), tenantUnread)
		if err != nil {
			return nil, err
		}
		return nil, s.cacheRepo.SafeReplaceSet(ctx, constant.GetBroadcastOpKey(tenantID, userID), opIDs)
	})

	if err != nil {
		return err
	}
	return nil
}

// ReconcileTenantBroadcastTotal 同步某租户全局广播 UserID 池
func (s *NotifyUnreadService) ReconcileTenantBroadcastTotal(ctx context.Context, tenantID string) error {
	sfKey := fmt.Sprintf("rec:total:%s", tenantID)

	_, err, _ := s.sfGroup.Do(sfKey, func() (interface{}, error) {
		totalIDs, err := s.dbRepo.CountBroadcastTotalByTenant(tenantID)
		if err != nil {
			return nil, err
		}
		err = s.cacheRepo.SafeReplaceSet(ctx, constant.GetBroadcastTotalKey(tenantID), totalIDs)
		if err != nil {
			return nil, err
		}
		return nil, nil
	})

	if err != nil {
		return err
	}
	return nil
}

// BatchReconcileUsers 批量同步用户非广播计数器
func (s *NotifyUnreadService) BatchReconcileUsers(ctx context.Context, tenantID string, uids []string, senderType int8) error {
	if len(uids) == 0 {
		return nil
	}

	var (
		counts map[string]int64
		err    error
	)

	if senderType == constant.SenderTypeTenant {
		counts, err = s.dbRepo.BatchCountUnreadTenant(tenantID, uids)
	} else {
		counts, err = s.dbRepo.BatchCountUnreadUser(tenantID, uids)
	}
	if err != nil {
		return err
	}

	kv := make(map[string]int64)
	for _, uid := range uids {
		var key string
		if senderType == constant.SenderTypeTenant {
			key = constant.GetUnreadTenantKey(tenantID, uid)
		} else {
			key = constant.GetUnreadPersonalKey(tenantID, uid)
		}
		kv[key] = counts[uid]
	}

	return s.cacheRepo.BatchSetPipeline(ctx, kv)
}

// ---  操作缓存相关值 ---

// IncrPersonalUnread 增加个人私信未读数
func (s *NotifyUnreadService) IncrPersonalUnread(ctx context.Context, tenantID, userID string) error {
	return s.cacheRepo.Incr(ctx, constant.GetUnreadPersonalKey(tenantID, userID))
}

// DecrPersonalUnread 减少个人私信未读数
func (s *NotifyUnreadService) DecrPersonalUnread(ctx context.Context, tenantID, userID string) (int64, error) {
	return s.cacheRepo.DecrWithFloor(ctx, constant.GetUnreadPersonalKey(tenantID, userID))
}

// SyncTenantBroadcastCacheToOp 同步用户的租户广播缓存到自己的广播操作池中
func (s *NotifyUnreadService) SyncTenantBroadcastCacheToOp(ctx context.Context, tenantID, userID string) (int64, error) {
	op, err := s.cacheRepo.SyncTenantBroadcastToOp(ctx, constant.GetBroadcastTotalKey(tenantID), constant.GetBroadcastOpKey(tenantID, userID))
	if err != nil {
		return 0, err
	}
	return op, nil
}

// ClearPersonUnread 清零个人私信未读数
func (s *NotifyUnreadService) ClearPersonUnread(ctx context.Context, tenantID string, userID string) error {
	return s.cacheRepo.Set(ctx, constant.GetUnreadPersonalKey(tenantID, userID), 0)
}

// IncrTenantUnread 增加租户非广播未读数
func (s *NotifyUnreadService) IncrTenantUnread(ctx context.Context, tenantID, userID string) error {
	return s.cacheRepo.Incr(ctx, constant.GetUnreadTenantKey(tenantID, userID))
}

// DecrTenantUnread 减少租户非广播未读数
func (s *NotifyUnreadService) DecrTenantUnread(ctx context.Context, tenantID, userID string) (int64, error) {
	return s.cacheRepo.DecrWithFloor(ctx, constant.GetUnreadTenantKey(tenantID, userID))
}

// TenantBroadcastIncr 处理租户新增广播
func (s *NotifyUnreadService) TenantBroadcastIncr(ctx context.Context, tenantID, notifyID string) (int64, error) {
	key := constant.GetBroadcastTotalKey(tenantID)

	_, err := s.cacheRepo.SAdd(ctx, key, notifyID)
	if err != nil {
		return 0, err
	}

	count, _ := s.cacheRepo.SCard(ctx, key)
	if count == 1 {
		if err := s.ReconcileTenantBroadcastTotal(ctx, tenantID); err != nil {
			return 0, fmt.Errorf("监测到租户总广播记录是新增的尝试进行一次同步，避免缓存单方面删除不同步问题: %w", err)
		}
	}
	return count, nil
}

// MarkUserBroadcastOp 记录用户对广播的已读或删除操作
func (s *NotifyUnreadService) MarkUserBroadcastOp(ctx context.Context, tenantID, userID, notifyID string) (int64, error) {
	return s.cacheRepo.SAdd(ctx, constant.GetBroadcastOpKey(tenantID, userID), notifyID)
}

// BatchIncrUserUnread 批量增加用户未读数
// 尝试原子增加 -> 收集不存在的 Key -> 批量从 DB 同步底数并回填
func (s *NotifyUnreadService) BatchIncrUserUnread(ctx context.Context, tenantID string, uids []string, senderType int8) error {
	if len(uids) == 0 {
		return nil
	}

	keys := make([]string, len(uids))
	for i, uid := range uids {
		if senderType == constant.SenderTypeTenant {
			keys[i] = constant.GetUnreadTenantKey(tenantID, uid)
		} else {
			keys[i] = constant.GetUnreadPersonalKey(tenantID, uid)
		}
	}

	results, err := s.cacheRepo.BatchIncrIfExist(ctx, keys)
	if err != nil {
		return fmt.Errorf("[Service] BatchIncrIfExist 失败: %w", err)
	}

	var needSyncIDs []string
	for i, res := range results {
		if res == -1 {
			needSyncIDs = append(needSyncIDs, uids[i])
		}
	}

	return s.BatchReconcileUsers(ctx, tenantID, uids, senderType)
}

type FullUnread struct {
	Personal int64
	Tenant   int64
}

// GetPersonalUnread  获取用户的个人未读
func (s *NotifyUnreadService) GetPersonalUnread(ctx context.Context, tenantID, userID string) (int64, error) {
	return s.cacheRepo.Get(ctx, constant.GetUnreadPersonalKey(tenantID, userID))
}

// GetTenantUnread 获取用户的租户未读
func (s *NotifyUnreadService) GetTenantUnread(ctx context.Context, tenantID, userID string) (int64, error) {
	noneBroadcastUnread, err := s.cacheRepo.Get(ctx, constant.GetUnreadTenantKey(tenantID, userID))
	if err != nil {
		return 0, err
	}
	broadcastUnread, err := s.cacheRepo.SDiffCount(ctx, constant.GetBroadcastTotalKey(tenantID), constant.GetBroadcastOpKey(tenantID, userID))
	if err != nil {
		return 0, err
	}
	return noneBroadcastUnread + broadcastUnread, nil
}

// GetFullUnread 获取用户完整未读数
func (s *NotifyUnreadService) GetFullUnread(ctx context.Context, tenantID, userID string) (FullUnread, error) {
	res, err := s.BatchGetFullUnread(ctx, tenantID, []string{userID})
	if err != nil {
		return FullUnread{}, err
	}
	return res[userID], nil
}

// BatchGetFullUnread 核心批量逻辑
func (s *NotifyUnreadService) BatchGetFullUnread(ctx context.Context, tenantID string, uids []string) (map[string]FullUnread, error) {
	if len(uids) == 0 {
		return nil, nil
	}

	allKeys := make([]string, 0, len(uids)*3)
	for _, uid := range uids {
		allKeys = append(allKeys,
			constant.GetUnreadPersonalKey(tenantID, uid),
			constant.GetUnreadTenantKey(tenantID, uid),
			constant.GetBroadcastOpKey(tenantID, uid),
		)
	}

	totalKey := constant.GetBroadcastTotalKey(tenantID)
	flatRes, err := s.cacheRepo.BatchGetUnreadDual(ctx, totalKey, allKeys)
	if err != nil {
		return nil, fmt.Errorf("批量获取未读数失败: %w", err)
	}

	result := make(map[string]FullUnread)
	for i, uid := range uids {
		result[uid] = FullUnread{
			Personal: flatRes[i*2],
			Tenant:   flatRes[i*2+1],
		}
	}
	return result, nil
}
