package service

import (
	"bs-notify-hub/internal/constant"
	"bs-notify-hub/internal/logic"
	"bs-notify-hub/internal/model"
	"bs-notify-hub/internal/repository"
	"bs-notify-hub/pkg/db"
	"context"
	"sync"
	"time"

	"bs-notify-hub/internal/conf"
	"bs-notify-hub/internal/dispatch"
	"bs-notify-hub/pkg/httpcode"
	"bs-notify-hub/pkg/response"

	"github.com/cloudwego/hertz/pkg/common/hlog"
	"gorm.io/gorm"
)

// SendNotifyProp 通知消息转发包装对象
type SendNotifyProp struct {
	Title      string
	Content    string
	TenantID   string
	SenderID   string
	SenderType int8
	EventType  string
	TTLSeconds *int64
	TargetType dispatch.TargetType
	TargetIDs  []string
}

// SendResult 统一的发送响应结果
type SendResult struct {
	NotifyID   string
	ExpireTime *time.Time
	TTLSeconds int64
}

func validateSenderType(senderType int8) bool {
	return senderType == constant.SenderTypeUser || senderType == constant.SenderTypeTenant
}

func validateSendProp(prop SendNotifyProp) *response.CodeError {
	if prop.TenantID == "" {
		return response.NewCodeError(httpcode.BadRequest, "tenantID 必填")
	}
	if !validateSenderType(prop.SenderType) {
		return response.NewCodeError(httpcode.BadRequest, "senderType 仅支持 0-用户 或 1-租户/系统")
	}
	if prop.TTLSeconds != nil && *prop.TTLSeconds < 0 {
		return response.NewCodeError(httpcode.BadRequest, "ttlSeconds 不能小于 0")
	}
	switch prop.TargetType {
	case dispatch.TargetSingle:
		if len(prop.TargetIDs) != 1 || prop.TargetIDs[0] == "" {
			return response.NewCodeError(httpcode.BadRequest, "一对一发送必须指定 1 个有效 userID")
		}
	case dispatch.TargetMultiple:
		if len(prop.TargetIDs) == 0 {
			return response.NewCodeError(httpcode.BadRequest, "一对多发送必须指定至少 1 个 userID")
		}
	case dispatch.TargetBroadcast:
		if len(prop.TargetIDs) != 0 {
			return response.NewCodeError(httpcode.BadRequest, "广播发送不能有目标用户")
		}
	}
	return nil
}

func resolveTTLSeconds(prop SendNotifyProp) (int64, *response.CodeError) {
	ttlSeconds := prop.TTLSeconds
	if ttlSeconds == nil {
		cfg := conf.GetConfig()
		var defaultTTL int64
		switch prop.SenderType {
		case constant.SenderTypeUser:
			defaultTTL = cfg.System.UserNotifyTTLSeconds
		case constant.SenderTypeTenant:
			defaultTTL = cfg.System.TenantNotifyTTLSeconds
		default:
			return 0, response.NewCodeError(httpcode.BadRequest, "senderType 不合法")
		}
		if defaultTTL < 0 {
			return 0, response.NewCodeError(httpcode.InternalError, "默认消息过期时间配置不能小于 0")
		}
		ttlSeconds = &defaultTTL
	}
	return *ttlSeconds, nil
}

func resolveExpireTime(ttlSeconds int64) *time.Time {
	if ttlSeconds == 0 {
		return nil
	}
	expireTime := time.Now().Add(time.Duration(ttlSeconds) * time.Second)
	return &expireTime
}

// NotifySenderService 通知中心业务逻辑核心
type NotifySenderService struct {
	repo *repository.NotifyRecordRepo
}

var (
	notifyInstance *NotifySenderService
	notifyOnce     sync.Once
)

// GetNotifySenderService 获取通知服务单例
func GetNotifySenderService() *NotifySenderService {
	notifyOnce.Do(func() {
		database := db.GetDB()
		notifyInstance = &NotifySenderService{
			repo: repository.NewNotifyRecordRepo(database),
		}
	})
	return notifyInstance
}

// SendToUser 一对一发送业务
func (s *NotifySenderService) SendToUser(ctx context.Context, prop SendNotifyProp) (*SendResult, *response.CodeError) {
	prop.TargetType = dispatch.TargetSingle
	return s.transmit(ctx, prop)
}

// SendToUsers 一对多发送业务
func (s *NotifySenderService) SendToUsers(ctx context.Context, prop SendNotifyProp) (*SendResult, *response.CodeError) {
	prop.TargetType = dispatch.TargetMultiple
	return s.transmit(ctx, prop)
}

// SendToAll 租户全员广播业务
func (s *NotifySenderService) SendToAll(ctx context.Context, prop SendNotifyProp) (*SendResult, *response.CodeError) {
	prop.TargetType = dispatch.TargetBroadcast
	return s.transmit(ctx, prop)
}

// transmit 通知分发点
func (s *NotifySenderService) transmit(ctx context.Context, prop SendNotifyProp) (*SendResult, *response.CodeError) {
	if err := validateSendProp(prop); err != nil {
		return nil, err
	}
	ttlSeconds, err := resolveTTLSeconds(prop)
	if err != nil {
		return nil, err
	}
	expireTime := resolveExpireTime(ttlSeconds)
	var expireAt *time.Time

	recordTargetIDs := prop.TargetIDs
	if prop.TargetType == dispatch.TargetBroadcast {
		recordTargetIDs = []string{}
	}

	record := model.NotifyRecord{
		Title:      prop.Title,
		Content:    prop.Content,
		TargetType: int8(prop.TargetType),
		SenderType: prop.SenderType,
		SenderID:   prop.SenderID,
		TenantID:   prop.TenantID,
		EventType:  prop.EventType,
		TargetIDs:  recordTargetIDs,
		ExpireAt:   expireAt,
	}
	txErr := db.GetDB().Transaction(func(tx *gorm.DB) error {
		recordRepo := repository.NewNotifyRecordRepo(tx)
		statusRepo := repository.NewNotifyStatusRepo(tx)

		if txErr := recordRepo.Create(&record); txErr != nil {
			return txErr
		}

		if prop.TargetType == dispatch.TargetSingle || prop.TargetType == dispatch.TargetMultiple {
			statuses := make([]*model.NotifyStatus, 0, len(prop.TargetIDs))
			for _, targetUserID := range prop.TargetIDs {
				statuses = append(statuses, &model.NotifyStatus{
					NotifyID:     record.NotifyID,
					TargetUserID: targetUserID,
					Status:       constant.NotifyStatusUnread,
				})
			}
			if len(statuses) > 0 {
				if txErr := statusRepo.CreateBulk(statuses); txErr != nil {
					return txErr
				}
			}
		}
		return nil
	})
	if txErr != nil {
		hlog.Errorf("发送的通知新增记录和状态失败: %v", err)
		err = response.NewCodeError(httpcode.InternalError, "发送的通知新增记录和状态失败")
		return nil, err
	}

	notifyID := record.NotifyID.String()
	logic.GetSender().SendNotify(ctx, record)
	return &SendResult{NotifyID: notifyID, ExpireTime: expireTime, TTLSeconds: ttlSeconds}, nil
}
