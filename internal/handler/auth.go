package handler

import (
	"context"
	"net/http"

	"bs-notify-hub/internal/conf"
	"bs-notify-hub/internal/middleware"
	"bs-notify-hub/pkg/httpcode"
	"bs-notify-hub/pkg/response"
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
		c.JSON(http.StatusBadRequest, response.Body{
			Code:    httpcode.BadRequest,
			Msg:     "请求参数错误",
			TraceID: response.GetTraceID(ctx),
		})
		return
	}

	cfg := conf.GetConfig()
	if body.Username != cfg.Auth.Admin.Username || body.Password != cfg.Auth.Admin.Password {
		c.JSON(http.StatusUnauthorized, response.Body{
			Code:    httpcode.Unauthorized,
			Msg:     "用户名或密码错误",
			TraceID: response.GetTraceID(ctx),
		})
		return
	}

	// 创建 session（管理员控制台会话不绑定租户，TenantID 为空串）
	sessionID := session.GetStore().Create("")

	// 设置 Cookie：有效期与 Session TTL 一致，HttpOnly，SameSite=Lax
	c.SetCookie(
		middleware.SessionCookieName,
		sessionID,
		session.TTLSeconds(), // MaxAge 秒
		"/",
		"",
		protocol.CookieSameSiteLaxMode,
		false, // 非 HTTPS 环境也能用，生产建议改 true
		true,  // HttpOnly
	)

	response.OkResp(ctx, c, "登录成功", nil)
}

// Logout POST /logout — 销毁 Session
func (h *AuthHandler) Logout(ctx context.Context, c *app.RequestContext) {
	sessionID := string(c.Cookie(middleware.SessionCookieName))
	if sessionID != "" {
		session.GetStore().Delete(sessionID)
	}
	// 清除 Cookie
	c.SetCookie(middleware.SessionCookieName, "", -1, "/", "", protocol.CookieSameSiteLaxMode, false, true)
	response.OkResp(ctx, c, "已登出", nil)
}
