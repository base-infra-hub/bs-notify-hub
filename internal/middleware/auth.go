package middleware

import (
	"context"
	"net/http"
	"strings"

	"bs-notify-hub/internal/conf"
	"bs-notify-hub/pkg/httpcode"
	"bs-notify-hub/pkg/jwtutil"
	"bs-notify-hub/pkg/response"
	"bs-notify-hub/pkg/session"

	"github.com/cloudwego/hertz/pkg/app"
)

// ContextKey 自定义 context 键类型，避免 context.WithValue 使用基础类型键
type ContextKey string

// TenantIDKey 租户 ID 的上下文键名
// HTTP 侧通过 c.Set(string(TenantIDKey), ...) 注入 RequestContext，
// gRPC 侧通过 context.WithValue 注入 ctx，两侧键名保持一致
const TenantIDKey ContextKey = "tenant_id"

// AuthMiddleware 双轨鉴权：第三方 JWT 与控制台 Session 均可访问
//  1. 优先校验 Authorization: Bearer <token>（RS256 验签 + tag 校验），
//     验签通过后从 claims 提取 tenant_id 注入上下文，禁止外部伪造租户标识
//  2. 否则回退校验 Cookie 中的 Session（控制台管理员会话），注入 Session 绑定的 tenant_id
//  3. 双轨均失败返回 401
func AuthMiddleware() app.HandlerFunc {
	return func(ctx context.Context, c *app.RequestContext) {
		// 1. JWT 轨道
		authHeader := string(c.GetHeader("Authorization"))
		if strings.HasPrefix(authHeader, "Bearer ") {
			cfg := conf.GetConfig()
			if pubKey := cfg.GetRSAPublicKey(); pubKey != nil {
				if claims, err := jwtutil.ValidateToken(authHeader, pubKey, cfg.Auth.ServiceTag); err == nil {
					// 兼容旧 token：claims 无 tenant_id 字段或类型异常时注入空串
					tenantID, _ := claims["tenant_id"].(string)
					c.Set(string(TenantIDKey), tenantID)
					c.Next(ctx)
					return
				}
			}
		}

		// 2. Session 轨道
		if cookie := c.Cookie(sessionCookieName); len(cookie) > 0 {
			if sess, ok := session.GetStore().Get(string(cookie)); ok {
				c.Set(string(TenantIDKey), sess.TenantID)
				c.Next(ctx)
				return
			}
		}

		c.JSON(http.StatusUnauthorized, response.Body{
			Code: httpcode.Unauthorized,
			Msg:  "未登录或凭证无效",
		})
		c.Abort()
	}
}

// SessionAuthMiddleware 仅允许控制台管理员 Session 会话（高危写操作专用）
func SessionAuthMiddleware() app.HandlerFunc {
	return func(ctx context.Context, c *app.RequestContext) {
		if cookie := c.Cookie(sessionCookieName); len(cookie) > 0 {
			if sess, ok := session.GetStore().Get(string(cookie)); ok {
				c.Set(string(TenantIDKey), sess.TenantID)
				c.Next(ctx)
				return
			}
		}
		c.JSON(http.StatusUnauthorized, response.Body{
			Code: httpcode.Unauthorized,
			Msg:  "权限不足：需要管理员会话",
		})
		c.Abort()
	}
}

// GetTenantID 从请求上下文提取中间件注入的租户 ID（未注入时返回空串）
func GetTenantID(c *app.RequestContext) string {
	if v, ok := c.Get(string(TenantIDKey)); ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

// ResolveTenantID 租户 ID 取值优先级：优先取中间件从可信凭证（JWT claims / Session）
// 注入的 tenant_id；取不到（空串）时回退请求体中的 reqTenantID。
// 回退仅为兼容旧 token（claims 无 tenant_id 字段）的过渡期逻辑。
func ResolveTenantID(c *app.RequestContext, reqTenantID string) string {
	if tid := GetTenantID(c); tid != "" {
		return tid
	}
	return reqTenantID
}
