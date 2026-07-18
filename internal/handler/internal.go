package handler

import (
	"context"
	"sync"

	"bs-notify-hub/internal/service"
	"bs-notify-hub/pkg/response"

	"github.com/cloudwego/hertz/pkg/app"
)

// InternalHandler 内部看板与健康检查接口
type InternalHandler struct {
	internalSvc *service.InternalService
}

var (
	internalHandlerInstance *InternalHandler
	internalHandlerOnce     sync.Once
)

// GetInternalHandler 获取内部处理器单例
func GetInternalHandler() *InternalHandler {
	internalHandlerOnce.Do(func() {
		internalHandlerInstance = &InternalHandler{
			internalSvc: service.GetInternalService(),
		}
	})
	return internalHandlerInstance
}

// Dashboard 返回服务器实时健康状态与运行指标
func (h *InternalHandler) Dashboard(ctx context.Context, c *app.RequestContext) {
	stats := h.internalSvc.Dashboard(ctx)
	response.OkResp(ctx, c, "获取看板数据成功", stats)
}
