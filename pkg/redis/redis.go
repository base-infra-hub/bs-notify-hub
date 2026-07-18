package redis

import (
	"context"
	"log"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

// Config Redis 基础接入配置 (通用定义)
type Config struct {
	Addrs    []string
	Password string
	DB       int
}

var (
	// 使用 UniversalClient 统一包装单机与集群客户端
	rdb  redis.UniversalClient
	once sync.Once
)

// InitRedis 初始化全局 Redis 客户端
func InitRedis(cfg Config) error {
	var err error
	once.Do(func() {
		rdb = redis.NewUniversalClient(&redis.UniversalOptions{
			Addrs:    cfg.Addrs,
			Password: cfg.Password,
			DB:       cfg.DB,
		})

		// 测试连接
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		_, err = rdb.Ping(ctx).Result()
		if err != nil {
			log.Printf("[Redis] 连接失败: %v", err)
			return
		}
		log.Printf("[Redis] 已成功连接至: %v", cfg.Addrs)
	})
	return err
}

// GetClient 获取客户端单例
func GetClient() redis.UniversalClient {
	if rdb == nil {
		log.Fatal("[Redis] 客户端尚未初始化，请先调用 InitRedis")
	}
	return rdb
}
