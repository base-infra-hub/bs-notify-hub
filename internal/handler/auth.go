package handler

import (
	"context"
	"net/http"
	"time"

	"bs-notify-hub/internal/conf"
	"bs-notify-hub/internal/middleware"
	"bs-notify-hub/pkg/session"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/protocol"
)

// AuthHandler 登录/登出处理器
type AuthHandler struct{}

var authHandlerInstance = &AuthHandler{}

func GetAuthHandler() *AuthHandler {
	return authHandlerInstance
}

// LoginPage GET /login — 返回登录页面（由 main.go 中静态路由托管，此处仅做跳转保护）
// 如果已有有效 session，直接跳回首页
func (h *AuthHandler) LoginPage(ctx context.Context, c *app.RequestContext) {
	sessionID := c.Cookie(middleware.SessionCookieName)
	if session.GetStore().Valid(string(sessionID)) {
		c.Redirect(http.StatusFound, []byte("/web"))
		return
	}
	// 返回登录页静态文件（由 serveDashboardStatic 处理，这里只做保护）
	c.Next(ctx)
}

// Login POST /login — 验证账号密码，成功后设置 Session Cookie
func (h *AuthHandler) Login(ctx context.Context, c *app.RequestContext) {
	var body struct {
		Username string `json:"username" form:"username"`
		Password string `json:"password" form:"password"`
	}
	if err := c.BindAndValidate(&body); err != nil {
		c.JSON(http.StatusBadRequest, map[string]interface{}{
			"code": 400,
			"msg":  "请求参数错误",
		})
		return
	}

	cfg := conf.GetConfig()
	if body.Username != cfg.Auth.Username || body.Password != cfg.Auth.Password {
		c.JSON(http.StatusUnauthorized, map[string]interface{}{
			"code": 401,
			"msg":  "用户名或密码错误",
		})
		return
	}

	// 创建 session
	sessionID := session.GetStore().Create()

	// 设置 Cookie：2小时过期，HttpOnly，SameSite=Lax
	c.SetCookie(
		middleware.SessionCookieName,
		sessionID,
		int(2*time.Hour/time.Second), // MaxAge 秒
		"/",
		"",
		protocol.CookieSameSiteLaxMode,
		false, // 非 HTTPS 环境也能用，生产建议改 true
		true,  // HttpOnly
	)

	c.JSON(http.StatusOK, map[string]interface{}{
		"code": 0,
		"msg":  "登录成功",
	})
}

// Logout POST /logout — 销毁 Session
func (h *AuthHandler) Logout(ctx context.Context, c *app.RequestContext) {
	sessionID := string(c.Cookie(middleware.SessionCookieName))
	if sessionID != "" {
		session.GetStore().Delete(sessionID)
	}
	// 清除 Cookie
	c.SetCookie(middleware.SessionCookieName, "", -1, "/", "", protocol.CookieSameSiteLaxMode, false, true)
	c.JSON(http.StatusOK, map[string]interface{}{
		"code": 0,
		"msg":  "已登出",
	})
}
