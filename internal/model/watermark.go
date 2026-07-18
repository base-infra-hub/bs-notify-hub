package model

import (
	"time"
)

// NotifyWatermark 用户通知状态水位线表 (主要用于标记租户通知的已读/删除基准)
type NotifyWatermark struct {
	// 租户ID
	TenantID string `gorm:"type:varchar(64);primaryKey;column:tenant_id"`

	// 用户ID
	UserID string `gorm:"type:varchar(64);primaryKey;column:user_id"`

	// 租户最后全量已读时间 (基准线，在此之前的租户通知视为已读)
	TenantReadAt time.Time `gorm:"column:tenant_read_at"`

	// 租户最后全量清空时间 (水位线，在此之前的租户通知视为已清空/删除)
	TenantClearAt time.Time `gorm:"column:tenant_clear_at"`
}

func (*NotifyWatermark) TableName() string {
	return "notify_watermark"
}
