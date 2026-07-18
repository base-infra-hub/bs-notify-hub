package logic

import (
	"bs-notify-hub/internal/constant"
	"bs-notify-hub/internal/dispatch"
	"bs-notify-hub/internal/dto"
	"bs-notify-hub/internal/model"
	"bs-notify-hub/pkg/response"
	"context"
	"encoding/json"
	"sync"
	"time"

	"github.com/cloudwego/hertz/pkg/common/hlog"
)

var (
	senderInstance *Sender
	senderOnce     sync.Once
)

type Sender struct {
	dispatcher *dispatch.Dispatcher
}

// GetSender 提供单例获取
func GetSender() *Sender {
	senderOnce.Do(func() {
		senderInstance = &Sender{
			dispatcher: dispatch.GetGlobal(),
		}
	})
	return senderInstance
}
func (s *Sender) SendTenantUnread(ctx context.Context, tenantID string, userId string, unread int64) {
	s.send(ctx, tenantID, dispatch.TargetSingle, []string{userId}, dto.InternalMsg{
		Type: dto.TenantUnreadMsg,
		Data: unread,
	})
}
func (s *Sender) SendPersonalUnread(ctx context.Context, tenantID string, userId string, unread int64) {
	s.send(ctx, tenantID, dispatch.TargetSingle, []string{userId}, dto.InternalMsg{
		Type: dto.PersonalUnreadMsg,
		Data: unread,
	})
}
func (s *Sender) SendNotify(ctx context.Context, record model.NotifyRecord) {
	traceID := response.GetTraceID(ctx)
	go func() {
		asyncCtx := response.SetTraceID(context.Background(), traceID)
		asyncCtx, cancel := context.WithTimeout(asyncCtx, 10*time.Second)
		defer cancel()
		notifyID := record.NotifyID.String()
		notifyUnreadService := GetNotifyUnreadService()

		// 1. 先更新对应类型未读数
		if record.TargetType == int8(dispatch.TargetBroadcast) {
			_, err := notifyUnreadService.TenantBroadcastIncr(asyncCtx, record.TenantID, notifyID)
			if err != nil {
				hlog.Errorf("新增租户广播消息失败[%s]，tenantID:%s，报错:%v", traceID, record.TenantID, err)
			}
		} else {
			err := notifyUnreadService.BatchIncrUserUnread(asyncCtx, record.TenantID, record.TargetIDs, record.SenderType)
			if err != nil {
				hlog.Errorf("记录非广播消息未读失败[%s]，tenantID:%s，报错:%v", traceID, record.TenantID, err)
			}
		}

		// 2. 推送最新未读总数（先推未读，后推通知内容）
		s.sendUnread(asyncCtx, record)

		// 3. 推送通知内容
		notifyData := dto.NotifyMsgData{
			NotifyID:   notifyID,
			Title:      record.Title,
			Content:    record.Content,
			TenantID:   record.TenantID,
			SenderID:   record.SenderID,
			SenderType: record.SenderType,
			TargetIDs:  record.TargetIDs,
			EventType:  record.EventType,
			ExpireTime: record.ExpireAt,
		}
		internalMsg := dto.InternalMsg{
			Type: dto.NotifyMsg,
			Data: notifyData,
		}
		if record.TargetType == int8(dispatch.TargetBroadcast) {
			s.send(asyncCtx, record.TenantID, dispatch.TargetBroadcast, nil, internalMsg)
		} else {
			s.send(asyncCtx, record.TenantID, dispatch.TargetSingle, record.TargetIDs, internalMsg)
		}
	}()
}

// sendUnread 根据通知类型推送对应未读总数
func (s *Sender) sendUnread(ctx context.Context, record model.NotifyRecord) {
	notifyUnreadService := GetNotifyUnreadService()
	if record.TargetType == int8(dispatch.TargetBroadcast) {
		// 广播消息：给租户下每个在线用户单独计算并推送 tenant 未读数
		onlineUserIDs := s.dispatcher.GetOnlineUserIDs(record.TenantID)
		for _, userID := range onlineUserIDs {
			unread, err := notifyUnreadService.GetTenantUnread(ctx, record.TenantID, userID)
			if err != nil {
				hlog.Errorf("[发送] 获取租户未读数失败，tenantID:%s，userID:%s，err:%v", record.TenantID, userID, err)
				continue
			}
			s.SendTenantUnread(ctx, record.TenantID, userID, unread)
		}
		return
	}

	// 非广播：按 SenderType 给每个目标用户推送对应类型未读数
	for _, userID := range record.TargetIDs {
		if record.SenderType == constant.SenderTypeUser {
			unread, err := notifyUnreadService.GetPersonalUnread(ctx, record.TenantID, userID)
			if err != nil {
				hlog.Errorf("[发送] 获取个人未读数失败，tenantID:%s，userID:%s，err:%v", record.TenantID, userID, err)
				continue
			}
			s.SendPersonalUnread(ctx, record.TenantID, userID, unread)
		} else {
			unread, err := notifyUnreadService.GetTenantUnread(ctx, record.TenantID, userID)
			if err != nil {
				hlog.Errorf("[发送] 获取租户未读数失败，tenantID:%s，userID:%s，err:%v", record.TenantID, userID, err)
				continue
			}
			s.SendTenantUnread(ctx, record.TenantID, userID, unread)
		}
	}
}
func (s *Sender) send(
	ctx context.Context,
	tenantID string,
	targetType dispatch.TargetType,
	targetIDs []string,
	msg dto.InternalMsg,
) {
	bytes, err := json.Marshal(msg)
	if err != nil {
		hlog.Errorf("消息序列化失败[%s]，报错:%v", response.GetTraceID(ctx), err)
	}
	traceID := response.GetTraceID(ctx)
	err = dispatch.GetGlobal().TransmitMessage(ctx, &dispatch.Message{
		TraceID:    traceID,
		TenantID:   tenantID,
		TargetType: targetType,
		TargetIDs:  targetIDs,
		Payload:    bytes,
	})
	if err != nil {
		hlog.Errorf("消息转发失败[%s]，报错:%v", traceID, err)
	}
}
