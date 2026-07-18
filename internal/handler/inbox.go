package handler

import (
	"bs-notify-hub/internal/dto"
	"bs-notify-hub/internal/service"
	"bs-notify-hub/pkg/httpcode"
	"bs-notify-hub/pkg/response"
	"context"
	"sync"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/common/hlog"
)

type InboxHandler struct {
	inboxSvc *service.NotifyInboxService
}

var (
	inboxHandlerInstance *InboxHandler
	inboxHandlerOnce     sync.Once
)

// GetInboxHandler 获取收件箱处理器单例
func GetInboxHandler() *InboxHandler {
	inboxHandlerOnce.Do(func() {
		inboxHandlerInstance = &InboxHandler{
			inboxSvc: service.GetNotifyInboxService(),
		}
	})
	return inboxHandlerInstance
}

// GetPersonalPage 查询个人收件箱分页数据
func (h *InboxHandler) GetPersonalPage(ctx context.Context, c *app.RequestContext) {
	var req dto.InboxPageReq
	if err := c.BindAndValidate(&req); err != nil {
		hlog.Errorf("参数验证失败: %v", err)
		response.ErrResp(ctx, c, response.NewCodeError(httpcode.BadRequest, "参数验证失败"))
		return
	}

	pageRes, err := h.inboxSvc.GetPersonalPageHttp(ctx, &req)
	if err != nil {
		response.ErrResp(ctx, c, err)
		return
	}
	response.OkResp(ctx, c, "查询成功", pageRes)
}

// GetTenantPage 查询租户收件箱分页数据（POST）
func (h *InboxHandler) GetTenantPage(ctx context.Context, c *app.RequestContext) {
	var req dto.InboxPageReq
	if err := c.BindAndValidate(&req); err != nil {
		hlog.Errorf("参数验证失败: %v", err)
		response.ErrResp(ctx, c, response.NewCodeError(httpcode.BadRequest, "参数验证失败"))
		return
	}

	pageRes, err := h.inboxSvc.GetTenantPageHttp(ctx, &req)
	if err != nil {
		response.ErrResp(ctx, c, err)
		return
	}
	response.OkResp(ctx, c, "查询成功", pageRes)
}
