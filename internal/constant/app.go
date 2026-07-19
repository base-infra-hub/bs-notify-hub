package constant

import (
	"fmt"

	"bs-notify-hub/pkg/redis"
)

const ServiceToken = "SHXY-BS-Notify-Hub"
const TraceIDKey = "trace_id"
const (
	SenderTypeUser   int8 = 0
	SenderTypeTenant int8 = 1
)

type NotifyCategory int8

const (
	NotifyCategoryUser   NotifyCategory = 0
	NotifyCategoryTenant NotifyCategory = 1
)

const (
	NotifyStatusUnread  int8 = 0
	NotifyStatusRead    int8 = 1
	NotifyStatusDeleted int8 = 2
)

// Redis Key 业务段前缀（不含服务命名空间；服务前缀 bs-notify-hub: 由 redis.WrapKey 在出口处统一拼装）
const (
	// KeyUnreadPersonalPrefix  1. 【用户类】用户个人私信未读数
	KeyUnreadPersonalPrefix = "unread:personal"

	// KeyUnreadTenantPrefix 2. 【租户类】非广播的租户发来的通知未读数
	KeyUnreadTenantPrefix = "unread:tenant"

	// KeyBroadcastTotalPrefix 3. 【租户类】租户广播总集合
	KeyBroadcastTotalPrefix = "broadcast:total"

	// KeyBroadcastOpPrefix  4. 【租户类】租户下用户已操作广播集合 (已读/已删除)
	KeyBroadcastOpPrefix = "broadcast:op"
)

/**
 * 用户Tenant未读数 = (【租户下的广播总数】 filter【租户下用户已操作(已读/删除)广播集合】) +【租户发送的非广播未读通知】
 * 用户Personal通知数 = 【用户未读通知】
 */

// GetUnreadPersonalKey GetUnreadSocialKey 获取【用户未读通知】 Key
func GetUnreadPersonalKey(tenantID, userID string) string {
	return redis.WrapKey(fmt.Sprintf("%s:%s:%s", KeyUnreadPersonalPrefix, tenantID, userID))
}

// GetUnreadTenantKey 获取【租户发送的非广播未读通知】 Key
func GetUnreadTenantKey(tenantID, userID string) string {
	return redis.WrapKey(fmt.Sprintf("%s:%s:%s", KeyUnreadTenantPrefix, tenantID, userID))
}

// GetBroadcastTotalKey 获取【租户下的广播集合】 Key
func GetBroadcastTotalKey(tenantID string) string {
	return redis.WrapKey(fmt.Sprintf("%s:%s", KeyBroadcastTotalPrefix, tenantID))
}

// GetBroadcastOpKey  获取【租户下用户已操作(已读/删除)广播集合】 Key
func GetBroadcastOpKey(tenantID, userID string) string {
	return redis.WrapKey(fmt.Sprintf("%s:%s:%s", KeyBroadcastOpPrefix, tenantID, userID))
}
