package handler

import (
	"bs-notify-hub/internal/dto"
	"bs-notify-hub/internal/middleware"
	"bs-notify-hub/internal/service"
	"bs-notify-hub/pkg/httpcode"
	"bs-notify-hub/pkg/response"
	"context"
	"sync"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/google/uuid"
)

// HubHandler 订阅中心处理器
type HubHandler struct {
	hubSvc    *service.HubService
	ticketSvc *service.TicketService
}

var (
	hubHandlerInstance *HubHandler
	hubHandlerOnce     sync.Once
)

// GetHubHandler 获取订阅中心处理器单例
func GetHubHandler() *HubHandler {
	hubHandlerOnce.Do(func() {
		hubHandlerInstance = &HubHandler{
			hubSvc:    service.GetHubService(),
			ticketSvc: service.GetTicketService(),
		}
	})
	return hubHandlerInstance
}

// Subscribe SSE 消息订阅接口
func (h *HubHandler) Subscribe(ctx context.Context, c *app.RequestContext) {
	ticket := c.Query("Ticket")
	ticketContent, err := h.ticketSvc.VerifyTicket(ctx, ticket)
	if err != nil {
		response.ErrResp(ctx, c, response.NewCodeError(httpcode.Unauthorized, err.Error()))
		return
	}

	tenantID := ticketContent.Tenant
	userID := ticketContent.UserID
	connID := uuid.New().String()

	h.hubSvc.Subscribe(ctx, c, &dto.SubscribeParam{
		TenantID: tenantID,
		UserID:   userID,
		ConnID:   connID,
	})
}

// ApplyTicket 申请凭证 REST 接口
func (h *HubHandler) ApplyTicket(ctx context.Context, c *app.RequestContext) {
	var req dto.ApplyTicketReq
	if err := c.BindAndValidate(&req); err != nil {

		return
	}
	tkt, exp, cre, err := h.ticketSvc.ApplyTicket(ctx, middleware.ResolveTenantID(c, req.Tenant), req.UserID)
	if err != nil {
		response.ErrResp(ctx, c, err)
		return
	}
	response.OkResp(ctx, c, "凭证申请成功", dto.ApplyTicketRes{
		Ticket:     tkt,
		ExpireTime: exp,
		CreateTime: cre,
	})
}
