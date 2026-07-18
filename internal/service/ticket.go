package service

import (
	"bs-notify-hub/internal/constant"
	"bs-notify-hub/pkg/httpcode"
	"bs-notify-hub/pkg/response"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"sync"
	"time"

	"bs-notify-hub/internal/conf"
	"bs-notify-hub/pkg/redis"

	"github.com/cloudwego/hertz/pkg/common/hlog"
)

const ticketRedisSpace string = constant.RedisSpace + ":ticket"

// TicketContent 凭证载体内容
type TicketContent struct {
	Tenant string `json:"tenant"`
	UserID string `json:"user_id"`
}

// TicketService 专门负责凭证生命周期管理（单例）
type TicketService struct{}

var (
	ticketInstance *TicketService
	ticketOnce     sync.Once
)

// GetTicketService 获取凭证服务单例
func GetTicketService() *TicketService {
	ticketOnce.Do(func() {
		ticketInstance = &TicketService{}
	})
	return ticketInstance
}

// ApplyTicket 申请订阅凭证逻辑 (AES 加密 + Redis 状态锁)
func (s *TicketService) ApplyTicket(ctx context.Context, tenant, userID string) (string, time.Time, time.Time, *response.CodeError) {
	ticketConfig := conf.GetConfig().Ticket
	expireSeconds := ticketConfig.ExpireSeconds
	aesKey := ticketConfig.AseKey

	now := time.Now()
	expireAt := time.Now().Add(time.Duration(expireSeconds) * time.Second)

	ticketContent := TicketContent{
		Tenant: tenant,
		UserID: userID,
	}

	// 1. 序列化
	contentJson, err := json.Marshal(ticketContent)
	if err != nil {
		hlog.Errorf("[Notify-Ticket] 无法序列化凭证内容: %v", err)
		return "", time.Time{}, time.Time{}, response.NewCodeError(httpcode.InternalError, "无法序列化凭证内容")
	}

	// 2. AES 加密生成 Base64 凭证
	ticket, err := encrypt(contentJson, []byte(aesKey))
	if err != nil {
		hlog.Errorf("[Notify-Ticket] 无法加密凭证内容: %v", err)
		return "", time.Time{}, time.Time{}, response.NewCodeError(httpcode.InternalError, "无法加密凭证内容")
	}

	// 3. 写入 Redis 缓存
	rdb := redis.GetClient()
	key := fmt.Sprintf("%s:%s", ticketRedisSpace, ticket)
	if err := rdb.Set(ctx, key, "1", time.Duration(expireSeconds)*time.Second).Err(); err != nil {
		hlog.Errorf("[Notify-Ticket] 写入redis缓存失败: %v", err)
		return "", time.Time{}, time.Time{}, response.NewCodeError(httpcode.InternalError, "保存凭证失败")
	}

	return ticket, expireAt, now, nil
}

// VerifyTicket 校验凭证真实性 (Redis 一次性预查 + AES 解密)
func (s *TicketService) VerifyTicket(ctx context.Context, ticket string) (*TicketContent, error) {
	if ticket == "" {
		return nil, fmt.Errorf("无效的凭证,凭证不能为空")
	}

	rdb := redis.GetClient()
	key := fmt.Sprintf("%s:%s", ticketRedisSpace, ticket)
	// 使用 GetDel 原子性地获取并从 Redis 中销毁
	_, err := rdb.GetDel(ctx, key).Result()
	if err != nil {
		hlog.Errorf("[Notify-Ticket] 获取凭证失败: %v", err)
		return nil, fmt.Errorf("凭证已失效或已被使用")
	}

	// 2. AES 解密
	content, err := decrypt(ticket, []byte(conf.GetConfig().Ticket.AseKey))
	if err != nil {
		return nil, fmt.Errorf("无法解密凭证内容: %v", err)
	}

	var ticketContent TicketContent
	err = json.Unmarshal(content, &ticketContent)
	if err != nil {
		return nil, fmt.Errorf("无法反序列化凭证内容: %v", err)
	}

	return &ticketContent, nil
}

// encrypt AES-GCM 加密辅助函数
func encrypt(plaintext, key []byte) (string, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}

	aesGCM, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	nonce := make([]byte, aesGCM.NonceSize())
	if _, err = io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}

	// 加密并将 nonce 附加在密文开头
	ciphertext := aesGCM.Seal(nonce, nonce, plaintext, nil)

	return base64.URLEncoding.EncodeToString(ciphertext), nil
}

// decrypt AES-GCM 解密辅助函数
func decrypt(ticket string, key []byte) ([]byte, error) {
	data, err := base64.URLEncoding.DecodeString(ticket)
	if err != nil {
		return nil, err
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	aesGCM, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	nonceSize := aesGCM.NonceSize()
	if len(data) < nonceSize {
		return nil, fmt.Errorf("密文数据长度不足")
	}
	nonce, ciphertext := data[:nonceSize], data[nonceSize:]

	return aesGCM.Open(nil, nonce, ciphertext, nil)
}
