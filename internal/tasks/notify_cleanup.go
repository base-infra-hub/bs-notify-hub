package tasks

import (
	"bs-notify-hub/internal/constant"
	"bs-notify-hub/internal/dispatch"
	"bs-notify-hub/internal/logic"
	"bs-notify-hub/internal/repository"
	"bs-notify-hub/pkg/db"
	"context"
	"log"
	"sync"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

const (
	notifyCleanupHour      = 3
	notifyCleanupBatchSize = 500
)

type NotifyCleanupService struct {
	db         *gorm.DB
	recordRepo *repository.NotifyRecordRepo
	statusRepo *repository.NotifyStatusRepo
}

var (
	notifyCleanupInstance *NotifyCleanupService
	notifyCleanupOnce     sync.Once
)

func GetNotifyCleanupService() *NotifyCleanupService {
	notifyCleanupOnce.Do(func() {
		notifyCleanupInstance = &NotifyCleanupService{
			db:         db.GetDB(),
			recordRepo: repository.NewNotifyRecordRepo(db.GetDB()),
			statusRepo: repository.NewNotifyStatusRepo(db.GetDB()),
		}
	})
	return notifyCleanupInstance
}

func (s *NotifyCleanupService) StartDaily(ctx context.Context) {
	go func() {
		for {
			delay := untilNextRun(time.Now())
			timer := time.NewTimer(delay)
			select {
			case <-ctx.Done():
				timer.Stop()
				return
			case <-timer.C:
				removed, err := s.RunOnce(ctx)
				if err != nil {
					log.Printf("[NotifyCleanup] 清理失败: %v", err)
					continue
				}
				log.Printf("[NotifyCleanup] 清理完成, 共删除过期通知及其状态 %d 条", removed)
			}
		}
	}()
}

// RunOnce 无限循环直至查不到过期数据
func (s *NotifyCleanupService) RunOnce(ctx context.Context) (int, error) {
	total := 0
	now := time.Now()

	for {
		select {
		case <-ctx.Done():
			return total, ctx.Err()
		default:
		}
		// 获取过期通知
		expireInfos, err := s.recordRepo.ListExpiredNotifyInfos(now, notifyCleanupBatchSize)
		if err != nil {
			return total, err
		}
		if len(expireInfos) == 0 {
			break
		}

		expireNotifyIDs := make([]uuid.UUID, len(expireInfos))
		for i := range expireInfos {
			expireNotifyIDs[i] = expireInfos[i].NotifyID
		}
		// 删除通知及其状态
		err = s.db.Transaction(func(tx *gorm.DB) error {
			if txErr := s.statusRepo.DeleteByNotifyIDs(expireNotifyIDs); txErr != nil {
				return txErr
			}
			if txErr := s.recordRepo.DeleteByNotifyIDs(expireNotifyIDs); txErr != nil {
				return txErr
			}
			return nil
		})

		if err != nil {
			return total, err
		}
		s.syncUnreadSnapshot(ctx, expireInfos)
		total += len(expireNotifyIDs)

		time.Sleep(time.Second * 1)

		if len(expireNotifyIDs) < notifyCleanupBatchSize {
			break
		}
	}

	return total, nil
}

func untilNextRun(now time.Time) time.Duration {
	next := time.Date(now.Year(), now.Month(), now.Day(), notifyCleanupHour, 0, 0, 0, now.Location())
	if !now.Before(next) {
		next = next.Add(24 * time.Hour)
	}
	return next.Sub(now)
}

func (s *NotifyCleanupService) syncUnreadSnapshot(ctx context.Context, infos []repository.ExpiredNotifyInfo) {
	unreadSvc := logic.GetNotifyUnreadService()

	personalTargets := make(map[string]map[string]struct{})
	tenantTargets := make(map[string]map[string]struct{})
	affectedTenantsForBroadcast := make(map[string]struct{})

	for _, info := range infos {
		targetType := dispatch.TargetType(info.TargetType)
		if targetType == dispatch.TargetBroadcast {
			affectedTenantsForBroadcast[info.TenantID] = struct{}{}
			continue
		}

		var tenantBucket map[string]map[string]struct{}
		if info.SenderType == constant.SenderTypeTenant {
			tenantBucket = tenantTargets
		} else {
			tenantBucket = personalTargets
		}

		if _, ok := tenantBucket[info.TenantID]; !ok {
			tenantBucket[info.TenantID] = make(map[string]struct{})
		}

		for _, userID := range info.TargetIDs {
			if userID == "" {
				continue
			}
			tenantBucket[info.TenantID][userID] = struct{}{}
		}
	}
	// 同步用户下个人未读计数器
	for tenantID, userSet := range personalTargets {
		userIDs := make([]string, 0, len(userSet))
		for userID := range userSet {
			userIDs = append(userIDs, userID)
		}
		if len(userIDs) == 0 {
			continue
		}
		if err := unreadSvc.BatchReconcileUsers(ctx, tenantID, userIDs, constant.SenderTypeUser); err != nil {
			log.Printf("[NotifyCleanup] 个人未读重算失败 tenant=%s err=%v", tenantID, err)
		}
	}
	// 同步用户下租户非广播未读计数器
	for tenantID, userSet := range tenantTargets {
		userIDs := make([]string, 0, len(userSet))
		for userID := range userSet {
			userIDs = append(userIDs, userID)
		}
		if len(userIDs) == 0 {
			continue
		}
		if err := unreadSvc.BatchReconcileUsers(ctx, tenantID, userIDs, constant.SenderTypeTenant); err != nil {
			log.Printf("[NotifyCleanup] 租户未读重算失败 tenant=%s err=%v", tenantID, err)
		}
	}
	// 同步受过期通知租户的总广播计数
	for tenantID := range affectedTenantsForBroadcast {
		if err := unreadSvc.ReconcileTenantBroadcastTotal(ctx, tenantID); err != nil {
			log.Printf("[NotifyCleanup] 广播总集合重算失败 tenant=%s err=%v", tenantID, err)
		}
	}
}
