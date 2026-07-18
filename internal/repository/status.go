package repository

import (
	"bs-notify-hub/internal/constant"
	"bs-notify-hub/internal/dispatch"
	"bs-notify-hub/internal/model"
	"errors"

	"github.com/google/uuid"

	"gorm.io/gorm"
)

type NotifyStatusRepo struct {
	db *gorm.DB
}

func NewNotifyStatusRepo(db *gorm.DB) *NotifyStatusRepo {
	return &NotifyStatusRepo{db: db}
}
func (r *NotifyStatusRepo) WithTx(db *gorm.DB) *NotifyStatusRepo {
	return &NotifyStatusRepo{db: db}
}
func (r *NotifyStatusRepo) Create(status *model.NotifyStatus) error {
	return r.db.Create(status).Error
}

func (r *NotifyStatusRepo) FindByID(statusNotifyID string) (*model.NotifyStatus, error) {
	var status model.NotifyStatus
	err := r.db.Where("status_notify_id = ?", statusNotifyID).First(&status).Error
	if err != nil {
		return nil, err
	}
	return &status, nil
}

func (r *NotifyStatusRepo) FindByUserAndNotify(userID string, notifyID uuid.UUID) (*model.NotifyStatus, error) {
	var status model.NotifyStatus
	err := r.db.Where("target_user_id = ? AND notify_id = ?", userID, notifyID).First(&status).Error
	if err != nil {
		return nil, err
	}
	return &status, nil
}

func (r *NotifyStatusRepo) FindByUser(userID string, limit, offset int) ([]model.NotifyStatus, error) {
	var statuses []model.NotifyStatus
	err := r.db.Where("target_user_id = ?", userID).Order("update_time DESC").Limit(limit).Offset(offset).Find(&statuses).Error
	return statuses, err
}

func (r *NotifyStatusRepo) CreateBulk(statuses []*model.NotifyStatus) error {
	return r.db.Create(statuses).Error
}

func (r *NotifyStatusRepo) UpsertByUserAndNotify(userID string, notifyID uuid.UUID, nextStatus int8) (*model.NotifyStatus, error) {
	status, err := r.FindByUserAndNotify(userID, notifyID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			created := &model.NotifyStatus{
				NotifyID:     notifyID,
				TargetUserID: userID,
				Status:       nextStatus,
			}
			if createErr := r.Create(created); createErr != nil {
				return nil, createErr
			}
			return created, nil
		}
		return nil, err
	}

	status.Status = nextStatus
	if err = r.Update(status); err != nil {
		return nil, err
	}
	return status, nil
}

func (r *NotifyStatusRepo) Update(status *model.NotifyStatus) error {
	return r.db.Save(status).Error
}

func (r *NotifyStatusRepo) Delete(statusNotifyID string) error {
	return r.db.Where("status_notify_id = ?", statusNotifyID).Delete(&model.NotifyStatus{}).Error
}

func (r *NotifyStatusRepo) DeleteByNotifyIDs(ids []uuid.UUID) error {
	if len(ids) == 0 {
		return nil
	}
	return r.db.Where("notify_id IN ?", ids).Delete(&model.NotifyStatus{}).Error
}

// BatchMarkReadByUser 一键标记个人所有通知为已读
func (r *NotifyStatusRepo) BatchMarkReadByUser(userID string, tenantID string) error {
	return r.db.Model(&model.NotifyStatus{}).
		Where("target_user_id = ?", userID).
		Where("notify_id IN (SELECT notify_id FROM notify_record WHERE tenant_id = ?)", tenantID).
		Update("status", constant.NotifyStatusRead).Error
}

// BatchMarkTenantNonBroadcastReadByUser 一键标记用户由租户发送的非广播通知为已读
func (r *NotifyStatusRepo) BatchMarkTenantNonBroadcastReadByUser(userID string, tenantID string) error {

	return r.db.Model(&model.NotifyStatus{}).
		Where("target_user_id = ?", userID).
		Where("notify_id IN (SELECT notify_id FROM notify_record WHERE tenant_id = ? AND target_type != ? AND sender_type = ?)", tenantID, dispatch.TargetBroadcast, constant.SenderTypeTenant).
		Update("status", constant.NotifyStatusRead).Error
}

// BatchMarkDeleteUserNotifyByUser 一键清空用户通知 (逻辑删除)
func (r *NotifyStatusRepo) BatchMarkDeleteUserNotifyByUser(userID string, tenantID string) error {
	return r.db.Model(&model.NotifyStatus{}).
		Where("target_user_id = ?", userID).
		Where("notify_id IN (SELECT notify_id FROM notify_record WHERE tenant_id = ?)", tenantID).
		Update("status", constant.NotifyStatusDeleted).Error
}

// BatchDeleteTenantNotifyByUser 一键标记由租户/系统发送的非广播通知为已删除
func (r *NotifyStatusRepo) BatchDeleteTenantNotifyByUser(userID string, tenantID string) error {
	return r.db.Model(&model.NotifyStatus{}).
		Where("target_user_id = ?", userID).
		Where("notify_id IN (SELECT notify_id FROM notify_record WHERE tenant_id = ? AND target_type != 2 AND sender_type = 1)", tenantID).
		Update("status", constant.NotifyStatusDeleted).Error
}
