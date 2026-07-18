package app

import (
	"fmt"
	"log"

	pkgdb "bs-notify-hub/pkg/db"
	pkgredis "bs-notify-hub/pkg/redis"

	"bs-notify-hub/internal/conf"
	"bs-notify-hub/internal/dispatch"
	"bs-notify-hub/pkg/pool"
)

func InitApp(cfg *conf.Config) error {
	log.Printf("[系统初始化] 正在加载基础组件...")
	if err := initRedis(cfg.Redis); err != nil {
		return err
	}
	if err := initDB(cfg.Database.Postgres); err != nil {
		return err
	}
	InitDispatcher(cfg.Dispatcher)
	log.Printf("[系统初始化] 业务组件加载完成 (DispatcherMode: %s)", cfg.Dispatcher.Mode)
	return nil
}

func initRedis(redisConfig conf.RedisConfig) error {
	redisCfg := pkgredis.Config{
		Addrs:    redisConfig.Addrs,
		Password: redisConfig.Password,
		DB:       redisConfig.DB,
	}
	if err := pkgredis.InitRedis(redisCfg); err != nil {
		return fmt.Errorf("[Redis] 初始化失败: %w", err)
	}
	return nil
}

func initDB(dbConfig conf.PostgresConfig) error {
	if err := pkgdb.InitDB(dbConfig); err != nil {
		return fmt.Errorf("[DB] 初始化失败: %w", err)
	}
	return nil
}

// InitDispatcher 初始化调度器
// 支持 local（本地内存通道）和 cluster（Redis Pub/Sub 广播集群）
func InitDispatcher(dispatchConfig conf.DispatcherConfig) {
	var broker dispatch.Broker
	switch dispatchConfig.Mode {
	case "cluster":
		broker = dispatch.NewRedisBroker(pkgredis.GetClient())
	default:
		broker = dispatch.NewLocalBroker(1000)
	}
	dispatcher := dispatch.NewDispatcher(broker, pool.NewManager())
	dispatcher.Run()
	dispatch.SetGlobal(dispatcher)
}
