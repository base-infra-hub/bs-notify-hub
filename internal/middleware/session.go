package middleware

import (
	"context"
	"net/http"

	"bs-notify-hub/pkg/session"

	"github.com/cloudwego/hertz/pkg/app"
)

const sessionCookieName = "sessionId"

// SessionMiddleware 校验请求必须携带有效的 Session Cookie
// 无效或缺失时：API 请求返回 401，页面请求重定向到 /login
func SessionMiddleware() app.HandlerFunc {
	return func(ctx context.Context, c *app.RequestContext) {
		sessionID := c.Cookie(sessionCookieName)
		if session.GetStore().Valid(string(sessionID)) {
			c.Next(ctx)
			return
		}

		// 判断是否是 API 请求（Accept: application/json 或 Ajax）
		accept := string(c.GetHeader("Accept"))
		xReq := string(c.GetHeader("X-Requested-With"))
		if isAPIRequest(accept, xReq) {
			c.JSON(http.StatusUnauthorized, map[string]interface{}{
				"code": 401,
				"msg":  "未登录或会话已过期，请重新登录",
			})
			c.Abort()
			return
		}

		// 页面请求：重定向到登录页
		c.Redirect(http.StatusFound, []byte("/web/login"))
		c.Abort()
	}
}

func isAPIRequest(accept, xRequestedWith string) bool {
	if xRequestedWith == "XMLHttpRequest" {
		return true
	}
	// fetch 请求通常带 application/json
	for _, v := range []string{"application/json", "text/plain", "*/*"} {
		if v == accept {
			return true
		}
	}
	return false
}

// SessionCookieName 导出 Cookie 名称供 handler 使用
const SessionCookieName = sessionCookieName

// HasValidSession 工具函数：判断请求是否携带有效 session（供 main.go 路由逻辑使用）
func HasValidSession(c *app.RequestContext) bool {
	sessionID := c.Cookie(sessionCookieName)
	return session.GetStore().Valid(string(sessionID))
}
