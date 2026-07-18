package model

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// NotifyStatus 用户通知状态表 (逻辑外键关联 NotifyRecord)
type NotifyStatus struct {
	// 状态记录ID
	StatusNotifyID uuid.UUID `gorm:"type:uuid;primaryKey;default:gen_random_uuid();column:status_notify_id"`

	// 通知内容ID
	NotifyID uuid.UUID `gorm:"type:uuid;index;column:notify_id"`

	// 目标用户ID (收件人)
	TargetUserID string `gorm:"type:varchar(64);index;column:target_user_id"`

	// 状态：0-未读, 1-已读, 2-已删除
	Status int8 `gorm:"type:smallint;default:0;column:status"`

	// 记录创建时间
	CreateTime time.Time `gorm:"autoCreateTime;column:create_time"`

	// 状态更新时间
	UpdateTime time.Time `gorm:"autoUpdateTime;column:update_time"`
}

func (*NotifyStatus) TableName() string {
	return "notify_status"
}

func (n *NotifyStatus) BeforeCreate(tx *gorm.DB) error {
	if n.StatusNotifyID == uuid.Nil {
		n.StatusNotifyID = uuid.New()
	}
	return nil
}
