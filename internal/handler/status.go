package handler

import (
	"context"
	"sync"

	"bs-notify-hub/internal/constant"
	"bs-notify-hub/internal/dto"
	"bs-notify-hub/internal/middleware"
	"bs-notify-hub/internal/service"
	"bs-notify-hub/pkg/httpcode"
	"bs-notify-hub/pkg/response"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/common/hlog"
)

// StatusHandler 负责通知状态相关的处理器，如已读/未读、删除等操作
type StatusHandler struct {
	statusSvc *service.NotifyStatusService
}

var (
	statusHandlerInstance *StatusHandler
	statusHandlerOnce     sync.Once
)

// GetStatusHandler 获取状态处理器单例
func GetStatusHandler() *StatusHandler {
	statusHandlerOnce.Do(func() {
		statusHandlerInstance = &StatusHandler{
			statusSvc: service.GetNotifyStatusService(),
		}
	})
	return statusHandlerInstance
}

// MarkRead 标记单条或指定通知已读
func (h *StatusHandler) MarkRead(ctx context.Context, c *app.RequestContext) {
	var req dto.OperateNotifyStatusReq
	if err := c.BindAndValidate(&req); err != nil {
		hlog.Errorf("参数验证失败: %v", err)
		response.ErrResp(ctx, c, response.NewCodeError(httpcode.BadRequest, "参数验证失败"))
		return
	}

	if err := h.statusSvc.MarkRead(ctx, req.NotifyID, req.UserID, middleware.ResolveTenantID(c, req.TenantID)); err != nil {
		response.ErrResp(ctx, c, err)
		return
	}
	response.OkResp(ctx, c, "标记已读成功", nil)
}

// BatchMarkRead 一键标记所有通知已读（全量操作）
func (h *StatusHandler) BatchMarkRead(ctx context.Context, c *app.RequestContext) {
	var req dto.BatchOperateNotifyStatusReq
	if err := c.BindAndValidate(&req); err != nil {
		hlog.Errorf("参数验证失败: %v", err)
		response.ErrResp(ctx, c, response.NewCodeError(httpcode.BadRequest, "参数验证失败"))
		return
	}

	var err *response.CodeError
	tenantID := middleware.ResolveTenantID(c, req.TenantID)
	if req.Type == int8(constant.NotifyCategoryTenant) {
		err = h.statusSvc.TenantBatchMarkRead(ctx, req.UserID, tenantID)
	} else {
		err = h.statusSvc.UserBatchMarkRead(ctx, req.UserID, tenantID)
	}

	if err != nil {
		response.ErrResp(ctx, c, err)
		return
	}
	response.OkResp(ctx, c, "一键已读成功", nil)
}

// DeleteNotify 删除单条通知（通常是逻辑删除）
func (h *StatusHandler) DeleteNotify(ctx context.Context, c *app.RequestContext) {
	var req dto.OperateNotifyStatusReq
	if err := c.BindAndValidate(&req); err != nil {
		hlog.Errorf("参数验证失败: %v", err)
		response.ErrResp(ctx, c, response.NewCodeError(httpcode.BadRequest, "参数验证失败"))
		return
	}

	if err := h.statusSvc.DeleteNotify(ctx, req.NotifyID, req.UserID, middleware.ResolveTenantID(c, req.TenantID)); err != nil {
		response.ErrResp(ctx, c, err)
		return
	}
	response.OkResp(ctx, c, "通知删除成功", nil)
}

// BatchDeleteNotify 一键清空收件箱
func (h *StatusHandler) BatchDeleteNotify(ctx context.Context, c *app.RequestContext) {
	var req dto.BatchOperateNotifyStatusReq
	if err := c.BindAndValidate(&req); err != nil {
		hlog.Errorf("参数验证失败: %v", err)
		response.ErrResp(ctx, c, response.NewCodeError(httpcode.BadRequest, "参数验证失败"))
		return
	}

	var err *response.CodeError
	tenantID := middleware.ResolveTenantID(c, req.TenantID)
	if req.Type == int8(constant.NotifyCategoryTenant) {
		err = h.statusSvc.TenantBatchDelete(ctx, req.UserID, tenantID)
	} else {
		err = h.statusSvc.UserBatchDelete(ctx, req.UserID, tenantID)
	}

	if err != nil {
		response.ErrResp(ctx, c, err)
		return
	}
	response.OkResp(ctx, c, "收件箱清空成功", nil)
}
