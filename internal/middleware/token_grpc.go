package middleware

import (
	"context"
	"strings"

	"bs-notify-hub/internal/conf"
	"bs-notify-hub/pkg/jwtutil"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

// TokenInterceptor 校验 gRPC 请求必须携带有效的 RSA JWT 服务令牌
// 从 metadata authorization 读取，格式：Bearer <token>
func TokenInterceptor() grpc.UnaryServerInterceptor {
	return func(
		ctx context.Context,
		req interface{},
		info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (interface{}, error) {
		cfg := conf.GetConfig()
		pubKey := cfg.GetRSAPublicKey()
		if pubKey == nil {
			return nil, status.Errorf(codes.Unauthenticated, "服务未配置 RSA 公钥，拒绝所有 JWT 请求")
		}

		md, ok := metadata.FromIncomingContext(ctx)
		if !ok {
			return nil, status.Errorf(codes.Unauthenticated, "缺少请求元数据")
		}

		values := md.Get("authorization")
		if len(values) == 0 {
			return nil, status.Errorf(codes.Unauthenticated, "缺少 authorization 元数据（格式：Bearer <token>）")
		}

		tokenStr := values[0]
		if !strings.HasPrefix(tokenStr, "Bearer ") {
			return nil, status.Errorf(codes.Unauthenticated, "authorization 格式错误（应为 Bearer <token>）")
		}

		_, err := jwtutil.ValidateToken(tokenStr, pubKey, cfg.Auth.ServiceTag)
		if err != nil {
			return nil, status.Errorf(codes.Unauthenticated, "无效的 JWT 令牌: %v", err)
		}

		return handler(ctx, req)
	}
}
