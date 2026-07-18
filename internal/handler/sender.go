package handler

import (
	"context"
	"sync"

	"bs-notify-hub/internal/dto"
	"bs-notify-hub/internal/service"
	"bs-notify-hub/pkg/httpcode"
	"bs-notify-hub/pkg/response"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/common/hlog"
)

// SenderHandler 负责发送/推送的处理器
type SenderHandler struct {
	svc *service.NotifySenderService
}

var (
	senderHandlerInstance *SenderHandler
	senderHandlerOnce     sync.Once
)

// GetSenderHandler 获取发送处理器单例
func GetSenderHandler() *SenderHandler {
	senderHandlerOnce.Do(func() {
		senderHandlerInstance = &SenderHandler{
			svc: service.GetNotifySenderService(),
		}
	})
	return senderHandlerInstance
}

// SendToUser 处理一对一发送
func (h *SenderHandler) SendToUser(ctx context.Context, c *app.RequestContext) {
	var req dto.SendToUserReq
	if err := c.BindAndValidate(&req); err != nil {
		hlog.Errorf("参数验证失败: %v", err)
		response.ErrResp(ctx, c, response.NewCodeError(httpcode.BadRequest, "参数验证失败"))
		return
	}

	prop := service.SendNotifyProp{
		Title:      req.Title,
		Content:    req.Content,
		TenantID:   req.TenantID,
		SenderID:   req.SenderID,
		SenderType: req.SenderType,
		EventType:  req.EventType,
		TTLSeconds: req.TTLSeconds,
		TargetIDs:  []string{req.UserID},
	}

	result, err := h.svc.SendToUser(ctx, prop)
	if err != nil {
		response.ErrResp(ctx, c, err)
		return
	}
	response.OkResp(ctx, c, "发送成功", dto.NotifyRes{
		NotifyID:   result.NotifyID,
		ExpireTime: result.ExpireTime,
		TTLSeconds: result.TTLSeconds,
	})
}

// SendToUsers 处理一对多发送
func (h *SenderHandler) SendToUsers(ctx context.Context, c *app.RequestContext) {
	var req dto.SendToUsersReq
	if err := c.BindAndValidate(&req); err != nil {
		hlog.Errorf("参数验证失败: %v", err)
		response.ErrResp(ctx, c, response.NewCodeError(httpcode.BadRequest, "参数验证失败"))
		return
	}

	prop := service.SendNotifyProp{
		Title:      req.Title,
		Content:    req.Content,
		TenantID:   req.TenantID,
		SenderID:   req.SenderID,
		SenderType: req.SenderType,
		EventType:  req.EventType,
		TTLSeconds: req.TTLSeconds,
		TargetIDs:  req.UserIDs,
	}

	result, err := h.svc.SendToUsers(ctx, prop)
	if err != nil {
		response.ErrResp(ctx, c, err)
		return
	}
	response.OkResp(ctx, c, "发送成功", dto.NotifyRes{
		NotifyID:   result.NotifyID,
		ExpireTime: result.ExpireTime,
		TTLSeconds: result.TTLSeconds,
	})
}

// Broadcast 处理全租户广播
func (h *SenderHandler) Broadcast(ctx context.Context, c *app.RequestContext) {
	var req dto.SendToAllReq
	if err := c.BindAndValidate(&req); err != nil {
		hlog.Errorf("参数验证失败: %v", err)
		response.ErrResp(ctx, c, response.NewCodeError(httpcode.BadRequest, "参数验证失败"))
		return
	}

	prop := service.SendNotifyProp{
		Title:      req.Title,
		Content:    req.Content,
		TenantID:   req.TenantID,
		SenderID:   req.SenderID,
		SenderType: req.SenderType,
		EventType:  req.EventType,
		TTLSeconds: req.TTLSeconds,
		TargetIDs:  []string{},
	}

	result, err := h.svc.SendToAll(ctx, prop)
	if err != nil {
		response.ErrResp(ctx, c, err)
		return
	}
	response.OkResp(ctx, c, "全员广播发送成功", dto.NotifyRes{
		NotifyID:   result.NotifyID,
		ExpireTime: result.ExpireTime,
		TTLSeconds: result.TTLSeconds,
	})
}
