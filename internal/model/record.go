package model

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

// NotifyRecord 通知内容表 (发件箱主体)
type NotifyRecord struct {
	// 全局唯一通知ID
	NotifyID uuid.UUID `gorm:"type:uuid;primaryKey;default:gen_random_uuid();column:notify_id"`

	// 通知标题
	Title string `gorm:"type:varchar(255);column:title"`

	// 通知内容 (使用 text 类型存储长文本)
	Content string `gorm:"type:text;column:content"`

	// 发送的类型: 0-单发, 1-群发, 2-全员广播
	TargetType int8 `gorm:"targetType:smallint;column:target_type"`

	// 发送方类型: 1-用户, 2-系统
	SenderType int8 `gorm:"type:smallint;column:sender_type"`

	// 发送人唯一标识
	SenderID string `gorm:"type:varchar(64);column:sender_id"`

	// 通知所属事件(上游自定义)
	EventType string `gorm:"type:varchar(64);column:event_type"`

	// 租户ID (多租户隔离)
	TenantID string `gorm:"type:varchar(64);index;column:tenant_id"`

	// 目标用户ID列表 (单发时仅有一个元素，群发时可有多个元素，全员广播时为空数组)
	TargetIDs datatypes.JSONSlice[string] `gorm:"type:jsonb;column:target_ids"`

	// 过期时间 (为空为永不过期，过期后可被清理)
	ExpireAt *time.Time `gorm:"column:expire_at;index"`

	// 创建时间
	CreatedAt time.Time `gorm:"autoCreateTime;column:created_at"`
}

func (*NotifyRecord) TableName() string {
	return "notify_record"
}

func (n *NotifyRecord) BeforeCreate(tx *gorm.DB) error {
	if n.NotifyID == uuid.Nil {
		n.NotifyID = uuid.New()
	}
	return nil
}
