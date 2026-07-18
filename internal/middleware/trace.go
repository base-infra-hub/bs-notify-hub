package middleware

import (
	"bs-notify-hub/pkg/response"
	"context"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/google/uuid"
)

// TraceMiddleware 全局链路追踪中间件
func TraceMiddleware() app.HandlerFunc {
	return func(ctx context.Context, c *app.RequestContext) {
		traceID := string(c.GetHeader("X-Trace-UserID"))
		if traceID == "" {
			traceID = uuid.New().String()
		}
		newCtx := response.SetTraceID(ctx, traceID)
		c.Header("X-Trace-UserID", traceID)
		c.Next(newCtx)
	}
}
