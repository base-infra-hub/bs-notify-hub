package repository

import (
	"bs-notify-hub/internal/model"
	"errors"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type NotifyWatermarkRepo struct {
	db *gorm.DB
}

func NewNotifyWatermarkRepo(db *gorm.DB) *NotifyWatermarkRepo {
	return &NotifyWatermarkRepo{db: db}
}
func (r *NotifyWatermarkRepo) WithTx(db *gorm.DB) *NotifyWatermarkRepo {
	return &NotifyWatermarkRepo{db: db}
}

// UpsertTenantReadAt 更新或插入用户的租户通知全量已读水位线
func (r *NotifyWatermarkRepo) UpsertTenantReadAt(tenantID, userID string, readAt time.Time) error {
	return r.db.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "tenant_id"}, {Name: "user_id"}},
		DoUpdates: clause.AssignmentColumns([]string{"tenant_read_at"}),
	}).Create(&model.NotifyWatermark{
		TenantID:     tenantID,
		UserID:       userID,
		TenantReadAt: readAt,
	}).Error
}

// UpsertTenantClearAt 更新或插入用户的租户通知全量清空水位线
func (r *NotifyWatermarkRepo) UpsertTenantClearAt(tenantID, userID string, clearAt time.Time) error {
	return r.db.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "tenant_id"}, {Name: "user_id"}},
		DoUpdates: clause.AssignmentColumns([]string{"tenant_clear_at"}),
	}).Create(&model.NotifyWatermark{
		TenantID:      tenantID,
		UserID:        userID,
		TenantClearAt: clearAt,
	}).Error
}

// FindByTenantAndUser 获取用户的水位线
func (r *NotifyWatermarkRepo) FindByTenantAndUser(tenantID, userID string) (*model.NotifyWatermark, error) {
	var record model.NotifyWatermark
	err := r.db.Where("tenant_id = ? AND user_id = ?", tenantID, userID).First(&record).Error

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &record, nil
}
