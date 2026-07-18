package middleware

import (
	"context"
	"net"
	"net/http"
	"strings"

	"bs-notify-hub/pkg/httpcode"
	"bs-notify-hub/pkg/response"

	"github.com/cloudwego/hertz/pkg/app"
)

// LocalhostOnly 仅允许本机地址访问（用于不对外暴露的内部接口）
func LocalhostOnly() app.HandlerFunc {
	return func(ctx context.Context, c *app.RequestContext) {
		clientIP := getClientIP(c)
		if !isLocalhost(clientIP) {
			c.JSON(http.StatusForbidden, response.Body{
				Code: httpcode.Forbidden,
				Msg:  "禁止访问：该接口仅限本机访问",
			})
			c.Abort()
			return
		}
		c.Next(ctx)
	}
}

func getClientIP(c *app.RequestContext) string {
	// 优先从 X-Forwarded-For 获取真实 IP（反向代理场景）
	xff := c.GetHeader("X-Forwarded-For")
	if len(xff) > 0 {
		parts := strings.Split(string(xff), ",")
		if len(parts) > 0 {
			return strings.TrimSpace(parts[0])
		}
	}

	xri := c.GetHeader("X-Real-Ip")
	if len(xri) > 0 {
		return string(xri)
	}

	return c.ClientIP()
}

func isLocalhost(ip string) bool {
	if ip == "" {
		return false
	}

	// IPv4 / IPv6 本机地址
	if ip == "127.0.0.1" || ip == "::1" || ip == "localhost" {
		return true
	}

	parsedIP := net.ParseIP(ip)
	if parsedIP == nil {
		return false
	}

	return parsedIP.IsLoopback()
}
