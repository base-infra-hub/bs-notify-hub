package response

import (
	"bs-notify-hub/internal/constant"
	"context"
	"log"
	"net/http"

	"bs-notify-hub/pkg/httpcode"

	"github.com/cloudwego/hertz/pkg/app"
)

// Body 统一的 API 响应包装结构
type Body struct {
	Code    httpcode.HttpCode `json:"code"`
	Msg     string            `json:"msg"`
	Data    interface{}       `json:"data"`
	TraceID string            `json:"traceId"`
}

// CodeError 定义包含业务状态码的自定义错误
type CodeError struct {
	Code httpcode.HttpCode `json:"code"`
	Msg  string            `json:"msg"`
}

func (e *CodeError) Error() string {
	return e.Msg
}

// NewCodeError 创建业务错误
func NewCodeError(code httpcode.HttpCode, msg string) *CodeError {
	return &CodeError{Code: code, Msg: msg}
}

// GetTraceID 从 context 中提取 TraceID
func GetTraceID(ctx context.Context) string {
	if traceID, ok := ctx.Value(constant.TraceIDKey).(string); ok {
		return traceID
	}
	return ""
}
func SetTraceID(ctx context.Context, traceID string) context.Context {
	return context.WithValue(ctx, constant.TraceIDKey, traceID)
}

// OkResp 成功响应
func OkResp(ctx context.Context, c *app.RequestContext, msg string, data interface{}) {
	c.JSON(http.StatusOK, Body{
		Code:    httpcode.Success,
		Msg:     msg,
		Data:    data,
		TraceID: GetTraceID(ctx), // 正确：从 ctx 获取
	})
}

// ErrResp 错误响应
func ErrResp(ctx context.Context, c *app.RequestContext, err *CodeError) {
	traceID := GetTraceID(ctx)

	log.Printf("[TraceID: %s] 详细错误记录: %v", traceID, err)

	c.JSON(http.StatusOK, Body{
		Code:    httpcode.InternalError,
		Msg:     err.Error(),
		Data:    nil,
		TraceID: traceID,
	})
}
