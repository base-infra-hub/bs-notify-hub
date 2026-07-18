package main

import (
	"embed"
	"io"
	"mime"
	"net/http"
	"path/filepath"
	"strings"

	"bs-notify-hub/internal/tasks"
	"context"
	"fmt"
	"log"
	"net"

	"bs-notify-hub/api/proto"
	"bs-notify-hub/internal/app"
	"bs-notify-hub/internal/conf"
	"bs-notify-hub/internal/handler"
	"bs-notify-hub/internal/logger"
	"bs-notify-hub/internal/middleware"
	"bs-notify-hub/internal/service"

	hertzApp "github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/app/server"
	"github.com/hertz-contrib/cors"
	"google.golang.org/grpc"
)

func main() {
	// 1. 加载配置并初始化应用层
	cfg, err := conf.LoadConfig()
	if err != nil {
		log.Printf("[配置错误] 加载配置失败: %v", err)
		waitAndExit()
		return
	}
	if err := logger.Init(); err != nil {
		log.Printf("[日志错误] 初始化日志失败: %v", err)
		waitAndExit()
		return
	}
	if err := app.InitApp(cfg); err != nil {
		log.Printf("[初始化错误] %v", err)
		waitAndExit()
		return
	}

	cfg.PrintConfig()

	if cfg.System.EnableNotifyCleanup {
		tasks.GetNotifyCleanupService().StartDaily(context.Background())
		log.Printf("[NotifyCleanup] 过期通知清理任务已启用")
	} else {
		log.Printf("[NotifyCleanup] 过期通知清理任务未启用 (system.enable_notify_cleanup=false)")
	}

	// 3. 启动 gRPC 服务（Notify 域负载）
	grpcServer := grpc.NewServer(
		grpc.UnaryInterceptor(middleware.TokenInterceptor()),
	)
	grpcNotifyHandler := service.GetGRPCNotifyService()
	proto.RegisterNotifyServiceServer(grpcServer, grpcNotifyHandler)

	go func() {
		addr := fmt.Sprintf("0.0.0.0:%d", cfg.Server.GRPCPort)
		lis, err := net.Listen("tcp", addr)
		if err != nil {
			log.Printf("[gRPC] 端口监听失败 (%s): %v", addr, err)
			waitAndExit()
			return
		}
		log.Printf("[gRPC] 通知服务运行于: %v", lis.Addr())
		if err := grpcServer.Serve(lis); err != nil {
			log.Printf("[gRPC] 启动失败: %v", err)
			waitAndExit()
		}
	}()

	// 4. 启动 Hertz HTTP 服务
	h := server.Default(server.WithHostPorts(fmt.Sprintf(":%d", cfg.Server.HTTPPort)))

	// 统一注册中间件（链路追踪全局启用；令牌校验与跨域下放到 /v1 路由组）
	h.Use(middleware.TraceMiddleware())

	// 5. 注册路由组（所有 HTTP API 均收敛到 /v1，看板接口也纳入 /v1 并统一支持跨域）
	registerRoutes(h)

	// 6. 注册嵌入式看板静态资源
	registerDashboardRoutes(h)

	log.Printf("[Hertz] HTTP 服务启动于端口 :%d", cfg.Server.HTTPPort)
	h.Spin()
}

//go:embed web/*
var webFS embed.FS

// serveDashboardStatic 服务嵌入的看板静态资源
func serveDashboardStatic(ctx context.Context, c *hertzApp.RequestContext) {
	path := string(c.Request.URI().Path())
	// 移除可能携带的 /web 前缀，转换为相对嵌入目录的路径
	path = strings.TrimPrefix(path, "/web")
	if path == "/" || path == "" {
		path = "/index.html"
	}

	// 仅服务看板相关静态资源，避免暴露其他路径
	if !isDashboardAsset(path) {
		c.SetStatusCode(404)
		c.SetBodyString("Not Found")
		return
	}

	cleanPath := strings.TrimPrefix(path, "/")
	file, err := webFS.Open("web/" + cleanPath)
	if err != nil {
		c.SetStatusCode(404)
		c.SetBodyString("Not Found")
		return
	}
	defer file.Close()

	content, err := io.ReadAll(file)
	if err != nil {
		c.SetStatusCode(500)
		c.SetBodyString("Internal Server Error")
		return
	}

	contentType := mime.TypeByExtension(filepath.Ext(cleanPath))
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	c.SetContentType(contentType)
	c.Write(content)
}

func isDashboardAsset(path string) bool {
	// 去除首部斜杠，进行匹配
	clean := strings.TrimPrefix(path, "/")
	return clean == "index.html" ||
		clean == "login.html" ||
		strings.HasPrefix(clean, "css/") ||
		strings.HasPrefix(clean, "js/") ||
		strings.HasPrefix(clean, "vendor/")
}

// registerDashboardRoutes 注册看板静态资源路由（全部收敛到 /web 下）
func registerDashboardRoutes(h *server.Hertz) {
	authHandler := handler.GetAuthHandler()
	internalHandler := handler.GetInternalHandler()

	// 根路径直接重定向到 /web/ 首页
	h.GET("/", func(ctx context.Context, c *hertzApp.RequestContext) {
		c.Redirect(http.StatusFound, []byte("/web"))
	})

	web := h.Group("/web")
	{
		// 登录页：GET 返回 login.html，POST 处理登录
		web.GET("/login", func(ctx context.Context, c *hertzApp.RequestContext) {
			// 已登录则跳回首页
			if middleware.HasValidSession(c) {
				c.Redirect(http.StatusFound, []byte("/web"))
				return
			}
			c.Request.SetRequestURI("/login.html")
			serveDashboardStatic(ctx, c)
		})
		web.POST("/login", authHandler.Login)
		web.POST("/logout", authHandler.Logout)

		// 静态资源（JS/CSS/Vendor）：无需 session，直接访问
		web.GET("/js/*filepath", serveDashboardStatic)
		web.GET("/css/*filepath", serveDashboardStatic)
		web.GET("/vendor/*filepath", serveDashboardStatic)

		// 看板主页和数据 API：需要 session 校验
		sessionGuard := middleware.SessionMiddleware()

		web.GET("/", sessionGuard, func(ctx context.Context, c *hertzApp.RequestContext) {
			c.Request.SetRequestURI("/index.html")
			serveDashboardStatic(ctx, c)
		})
		web.GET("/index.html", sessionGuard, func(ctx context.Context, c *hertzApp.RequestContext) {
			serveDashboardStatic(ctx, c)
		})

		// 看板数据 API 接口
		web.GET("/dashboard", sessionGuard, internalHandler.Dashboard)
	}

	// favicon 避免 404
	h.GET("/favicon.ico", func(ctx context.Context, c *hertzApp.RequestContext) {
		c.SetStatusCode(204)
	})
}

// waitAndExit 打印提示信息并等待用户按键后退出
func waitAndExit() {
	fmt.Println("\n按任意键退出...")
	fmt.Scanln()
}

// registerRoutes 提取路由注册逻辑，使主流程更清晰
func registerRoutes(h *server.Hertz) {
	statusHandler := handler.GetStatusHandler()
	hubHandler := handler.GetHubHandler()
	senderHandler := handler.GetSenderHandler()
	inboxHandler := handler.GetInboxHandler()

	// /v1 统一挂载跨域中间件 + JWT 令牌校验（全部业务接口）
	v1 := h.Group("/v1", cors.Default(), middleware.TokenMiddleware())
	{
		sender := v1.Group("/sender")
		{
			sender.POST("/user", senderHandler.SendToUser)
			sender.POST("/users", senderHandler.SendToUsers)
			sender.POST("/all", senderHandler.Broadcast)
		}
		inbox := v1.Group("/inbox")
		{
			inbox.POST("/personal", inboxHandler.GetPersonalPage)
			inbox.POST("/tenant", inboxHandler.GetTenantPage)
		}
		state := v1.Group("/status")
		{
			state.POST("/read", statusHandler.MarkRead)
			state.POST("/read/all", statusHandler.BatchMarkRead)
			state.DELETE("/delete", statusHandler.DeleteNotify)
			state.DELETE("/delete/all", statusHandler.BatchDeleteNotify)
		}
		hub := v1.Group("/hub")
		{
			// ticket/apply：申请凭证，需要 JWT（第三方服务调用）
			hub.POST("/ticket/apply", hubHandler.ApplyTicket)
		}
	}

	// subscribe：SSE 长连接，浏览器直连，不能携带 Authorization header
	// 使用 Ticket 机制二次鉴权（先调 ticket/apply 拿到一次性凭证，再用凭证建立连接）
	h.GET("/v1/hub/subscribe", cors.Default(), hubHandler.Subscribe)
}
