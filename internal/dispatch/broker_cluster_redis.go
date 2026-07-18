package dispatch

import (
	"context"
	"encoding/json"
	"log"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

const defaultRedisDispatchChannel = "notify_hub_dispatch"

type RedisBroker struct {
	rdb     redis.UniversalClient
	channel string
	outChan chan *Message
	ctx     context.Context
	cancel  context.CancelFunc
	subOnce sync.Once
}

// NewRedisBroker 接收统一的 UniversalClient 客户端，复用连接池
func NewRedisBroker(rdb redis.UniversalClient) *RedisBroker {
	ctx, cancel := context.WithCancel(context.Background())

	return &RedisBroker{
		rdb:     rdb,
		channel: defaultRedisDispatchChannel,
		outChan: make(chan *Message, 1000),
		ctx:     ctx,
		cancel:  cancel,
	}
}

func (r *RedisBroker) Publish(msg *Message) error {
	data, err := json.Marshal(msg)
	if err != nil {
		return err
	}
	return r.rdb.Publish(r.ctx, r.channel, data).Err()
}

func (r *RedisBroker) Subscribe() <-chan *Message {
	r.subOnce.Do(func() {
		go r.consumeLoop()
	})
	return r.outChan
}

func (r *RedisBroker) consumeLoop() {
	for {
		select {
		case <-r.ctx.Done():
			log.Println("[RedisBroker] 停止订阅")
			return
		default:
		}

		err := r.consumeOnce()
		if err != nil {
			log.Println("[RedisBroker] 订阅异常，准备重试:", err)
			time.Sleep(2 * time.Second)
		}
	}
}

func (r *RedisBroker) consumeOnce() error {
	sub := r.rdb.Subscribe(r.ctx, r.channel)
	defer sub.Close()

	ch := sub.Channel()

	log.Println("[RedisBroker] 已订阅 channel:", r.channel)

	for {
		select {
		case <-r.ctx.Done():
			return nil

		case msg, ok := <-ch:
			if !ok {
				return redis.ErrClosed
			}

			var m Message
			if err := json.Unmarshal([]byte(msg.Payload), &m); err != nil {
				log.Println("消息解析失败:", err)
				continue
			}

			// 防止阻塞
			select {
			case r.outChan <- &m:
			default:
				log.Println("[RedisBroker] outChan 已满，丢弃消息:", m)
			}
		}
	}
}

func (r *RedisBroker) Close() {
	r.cancel()
}
