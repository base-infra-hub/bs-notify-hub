package service

import (
	"bs-notify-hub/internal/constant"
	"bs-notify-hub/internal/dispatch"
	"bs-notify-hub/internal/logic"
	"bs-notify-hub/internal/model"
	"bs-notify-hub/internal/repository"
	"bs-notify-hub/pkg/db"
	"bs-notify-hub/pkg/httpcode"
	"bs-notify-hub/pkg/response"
	"context"
	"errors"
	"runtime/debug"
	"sync"
	"time"

	"github.com/cloudwego/hertz/pkg/common/hlog"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type BatchOpType int

// NotifyStatusService 负责通知已读/删除等状态变更
type NotifyStatusService struct {
	db            *gorm.DB
	recordRepo    *repository.NotifyRecordRepo
	statusRepo    *repository.NotifyStatusRepo
	watermarkRepo *repository.NotifyWatermarkRepo
	unreadService *logic.NotifyUnreadService
	sender        *logic.Sender
}

var (
	notifyStatusInstance *NotifyStatusService
	notifyStatusOnce     sync.Once
)

// GetNotifyStatusService 获取通知状态服务单例
func GetNotifyStatusService() *NotifyStatusService {
	notifyStatusOnce.Do(func() {
		database := db.GetDB()
		notifyStatusInstance = &NotifyStatusService{
			db:            database,
			recordRepo:    repository.NewNotifyRecordRepo(database),
			statusRepo:    repository.NewNotifyStatusRepo(database),
			watermarkRepo: repository.NewNotifyWatermarkRepo(database),
			unreadService: logic.GetNotifyUnreadService(),
			sender:        logic.GetSender(),
		}
	})
	return notifyStatusInstance
}

/*
 * 通知的单向操作
 */

// MarkRead 标记已读
func (s *NotifyStatusService) MarkRead(ctx context.Context, notifyID, userID, tenantID string) *response.CodeError {
	return s.execStatusUpdate(ctx, notifyID, userID, tenantID, constant.NotifyStatusRead)
}

// DeleteNotify 删除通知
func (s *NotifyStatusService) DeleteNotify(ctx context.Context, notifyID, userID, tenantID string) *response.CodeError {
	return s.execStatusUpdate(ctx, notifyID, userID, tenantID, constant.NotifyStatusDeleted)
}

// execStatusUpdate 处理通知状态变更
func (s *NotifyStatusService) execStatusUpdate(ctx context.Context, notifyIDStr, userID, tenantID string, nextStatus int8) *response.CodeError {
	record, funErr := s.validateNotifyAccess(ctx, notifyIDStr, userID, tenantID)
	if funErr != nil {
		return funErr
	}
	_, dbErr := s.statusRepo.UpsertByUserAndNotify(userID, record.NotifyID, nextStatus)
	if dbErr != nil {
		hlog.Errorf("[通知状态] 更新失败: err=%v", dbErr)
		return response.NewCodeError(httpcode.InternalError, "更新通知状态失败")
	}
	s.asyncUpdateOnceOpCacheAndSend(ctx, record, userID)

	return nil
}

/*
 * 通知一键操作
 */

// UserBatchMarkRead 用户侧一键已读：标记该用户在当前租户下所有已下发的通知状态为已读
func (s *NotifyStatusService) UserBatchMarkRead(ctx context.Context, userID, tenantID string) *response.CodeError {
	err := s.statusRepo.BatchMarkReadByUser(userID, tenantID)
	if err != nil {
		hlog.Errorf("[通知状态] 批量更新通知已读状态失败: err=%v", err)
		return response.NewCodeError(httpcode.InternalError, "批量更新通知已读状态失败")
	}
	s.asyncUpdateBatchOpCacheAndSend(ctx, userID, tenantID, constant.NotifyCategoryUser)
	return nil
}

// TenantBatchMarkRead 租户侧一键已读：更新租户通知基准线时间，并标记租户发送的非广播通知为已读
func (s *NotifyStatusService) TenantBatchMarkRead(ctx context.Context, userID, tenantID string) *response.CodeError {
	dbErr := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := s.watermarkRepo.WithTx(tx).UpsertTenantReadAt(tenantID, userID, time.Now()); err != nil {
			return err
		}
		if err := s.statusRepo.WithTx(tx).BatchMarkTenantNonBroadcastReadByUser(userID, tenantID); err != nil {
			return err
		}
		return nil
	})
	if dbErr != nil {
		hlog.Errorf("[通知状态] 更新用户通知已读状态和水位线事物失败失败: err=%v", dbErr)
		return response.NewCodeError(httpcode.InternalError, "更新用户通知已读状态和水位线失败")
	}
	s.asyncUpdateBatchOpCacheAndSend(ctx, userID, tenantID, constant.NotifyCategoryTenant)
	return nil
}

// UserBatchDelete 用户侧一键清空：标记该用户在当前租户下所有通知为已删除状态
func (s *NotifyStatusService) UserBatchDelete(ctx context.Context, userID, tenantID string) *response.CodeError {
	err := s.statusRepo.BatchMarkDeleteUserNotifyByUser(userID, tenantID)
	if err != nil {
		hlog.Errorf("[通知状态] 批量更新通知已删除状态失败: err=%v", err)
		return response.NewCodeError(httpcode.InternalError, "批量更新通知已删除状态失败")
	}
	s.asyncUpdateBatchOpCacheAndSend(ctx, userID, tenantID, constant.NotifyCategoryUser)
	return nil
}

// TenantBatchDelete 租户侧一键清空：更新租户清空水位线，并标记租户发送的非广播通知为已删除
func (s *NotifyStatusService) TenantBatchDelete(ctx context.Context, userID, tenantID string) *response.CodeError {
	dbErr := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := s.watermarkRepo.WithTx(tx).UpsertTenantClearAt(tenantID, userID, time.Now()); err != nil {
			return err
		}
		if err := s.statusRepo.WithTx(tx).BatchMarkDeleteUserNotifyByUser(userID, tenantID); err != nil {
			return err
		}
		return nil
	})
	if dbErr != nil {
		hlog.Errorf("[通知状态] 批量更新通知已删除状态和水位线事物失败失败: err=%v", dbErr)
		return response.NewCodeError(httpcode.InternalError, "批量更新通知已删除状态和水位线失败")
	}
	s.asyncUpdateBatchOpCacheAndSend(ctx, userID, tenantID, constant.NotifyCategoryTenant)
	return nil
}

// asyncUpdateBatchOpCacheAndSend 异步更新批量操作后的缓存，并推送对应影响到的通知类型的未读变更
func (s *NotifyStatusService) asyncUpdateBatchOpCacheAndSend(ctx context.Context, userID, tenantID string, notifyCategory constant.NotifyCategory) {
	traceID := response.GetTraceID(ctx)
	go func() {
		asyncCtx := response.SetTraceID(context.Background(), traceID)
		asyncCtx, cancel := context.WithTimeout(asyncCtx, 10*time.Second)
		defer func() {
			if r := recover(); r != nil {
				hlog.Errorf("[通知状态] 异步更新失败! TraceID: %s, 内容: %v\n%s", traceID, r, debug.Stack())
			}
			cancel()
		}()
		if notifyCategory == constant.NotifyCategoryTenant {
			_, err := s.unreadService.SyncTenantBroadcastCacheToOp(asyncCtx, tenantID, userID)
			if err != nil {
				hlog.Errorf("[通知状态] 批量更新租户广播缓存失败: %v", err)
				return
			}
			unread, err := s.unreadService.GetTenantUnread(asyncCtx, tenantID, userID)
			if err != nil {
				hlog.Errorf("[通知状态] 批量更新租户未读缓存失败: %v", err)
				return
			}
			s.sender.SendTenantUnread(asyncCtx, tenantID, userID, unread)
			return
		}
		if notifyCategory == constant.NotifyCategoryUser {
			err := s.unreadService.ClearPersonUnread(asyncCtx, tenantID, userID)
			if err != nil {
				hlog.Errorf("[通知状态] 批量更新个人未读缓存失败: %v", err)
				return
			}
			s.sender.SendPersonalUnread(asyncCtx, tenantID, userID, 0)
		}
	}()
}

// asyncUpdateOnceOpCacheAndSend 更新缓存并推送对应影响到的通知类型的未读变更
func (s *NotifyStatusService) asyncUpdateOnceOpCacheAndSend(ctx context.Context, record *model.NotifyRecord, userID string) {
	traceID := response.GetTraceID(ctx)
	go func() {
		asyncCtx := response.SetTraceID(context.Background(), traceID)
		asyncCtx, cancel := context.WithTimeout(asyncCtx, 10*time.Second)
		defer func() {
			if r := recover(); r != nil {
				hlog.Errorf("[通知状态] 异步更新缓存和推送失败: %v", r)
			}
			cancel()
		}()
		if record.TargetType == int8(dispatch.TargetBroadcast) {
			_, err := s.unreadService.MarkUserBroadcastOp(asyncCtx, record.TenantID, userID, record.NotifyID.String())
			if err != nil {
				hlog.Errorf("[通知状态] 更新租户广播操作缓存失败: %v", err)
				return
			}
			tenant, err := s.unreadService.GetTenantUnread(asyncCtx, record.TenantID, userID)
			if err != nil {
				hlog.Errorf("[通知状态] 获取租户未读缓存失败: %v", err)
				return
			}
			s.sender.SendTenantUnread(asyncCtx, record.TenantID, userID, tenant)
			return
		}
		if record.SenderType == constant.SenderTypeUser {
			val, err := s.unreadService.DecrPersonalUnread(asyncCtx, record.TenantID, userID)
			if err != nil {
				hlog.Errorf("[通知状态] 更新个人未读缓存失败: %v", err)
				return
			}
			s.sender.SendPersonalUnread(asyncCtx, record.TenantID, userID, val)
		} else {
			val, err := s.unreadService.DecrTenantUnread(asyncCtx, record.TenantID, userID)
			if err != nil {
				hlog.Errorf("[通知状态] 更新租户非广播未读缓存失败: %v", err)
				return
			}
			s.sender.SendTenantUnread(asyncCtx, record.TenantID, userID, val)
		}
	}()
}

// validateNotifyAccess 验证用户是否有权操作该通知
func (s *NotifyStatusService) validateNotifyAccess(ctx context.Context, notifyID, userID, tenantID string) (*model.NotifyRecord, *response.CodeError) {
	if notifyID == "" || userID == "" || tenantID == "" {
		hlog.Errorf("[通知状态] 参数错误: notifyId=%s, userId=%s, tenantId=%s", notifyID, userID, tenantID)
		return nil, response.NewCodeError(httpcode.BadRequest, "notifyId/userId/tenantId 必填")
	}

	if _, err := uuid.Parse(notifyID); err != nil {
		return nil, response.NewCodeError(httpcode.BadRequest, "notifyId 格式不正确")
	}

	record, err := s.recordRepo.FindByID(notifyID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			hlog.Errorf("[通知状态] 通知不存在: notifyId=%s", notifyID)
			return nil, response.NewCodeError(httpcode.NotFound, "通知不存在")
		}
		hlog.Errorf("[通知状态] 查询通知失败: notifyId=%s, err=%v", notifyID, err)
		return nil, response.NewCodeError(httpcode.InternalError, "查询通知失败")
	}

	if record.TenantID != tenantID {
		hlog.Errorf("[通知状态] 无权操作该通知: notifyId=%s, userId=%s, tenantId=%s", notifyID, userID, tenantID)
		return nil, response.NewCodeError(httpcode.Forbidden, "无权操作该通知")
	}

	targetType := dispatch.TargetType(record.TargetType)
	if targetType == dispatch.TargetSingle || targetType == dispatch.TargetMultiple {
		if !containsUser(record.TargetIDs, userID) {
			hlog.Errorf("[通知状态] 无权操作该通知: notifyId=%s, userId=%s, tenantId=%s", notifyID, userID, tenantID)
			return nil, response.NewCodeError(httpcode.Forbidden, "无权操作该通知")
		}
	}

	return record, nil
}

func containsUser(userIDs []string, target string) bool {
	for _, userID := range userIDs {
		if userID == target {
			return true
		}
	}
	return false
}
