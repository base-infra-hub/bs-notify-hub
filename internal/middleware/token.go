package middleware

import (
	"context"
	"net/http"
	"strings"

	"bs-notify-hub/internal/conf"
	"bs-notify-hub/pkg/httpcode"
	"bs-notify-hub/pkg/jwtutil"
	"bs-notify-hub/pkg/response"

	"github.com/cloudwego/hertz/pkg/app"
)

// TokenMiddleware 校验请求必须携带有效的 RSA JWT 服务令牌
// 从 Authorization 请求头读取，格式：Bearer <token>
func TokenMiddleware() app.HandlerFunc {
	return func(ctx context.Context, c *app.RequestContext) {
		cfg := conf.GetConfig()
		pubKey := cfg.GetRSAPublicKey()
		if pubKey == nil {
			c.JSON(http.StatusUnauthorized, response.Body{
				Code: httpcode.Unauthorized,
				Msg:  "服务未配置 RSA 公钥，拒绝所有 JWT 请求",
			})
			c.Abort()
			return
		}

		authHeader := string(c.GetHeader("Authorization"))
		if authHeader == "" || !strings.HasPrefix(authHeader, "Bearer ") {
			c.JSON(http.StatusUnauthorized, response.Body{
				Code: httpcode.Unauthorized,
				Msg:  "缺少 Authorization 请求头（格式：Bearer <token>）",
			})
			c.Abort()
			return
		}

		_, err := jwtutil.ValidateToken(authHeader, pubKey, cfg.Auth.ServiceTag)
		if err != nil {
			c.JSON(http.StatusUnauthorized, response.Body{
				Code: httpcode.Unauthorized,
				Msg:  "无效的 JWT 令牌: " + err.Error(),
			})
			c.Abort()
			return
		}

		c.Next(ctx)
	}
}
