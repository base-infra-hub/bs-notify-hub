package repository

import (
	"bs-notify-hub/internal/constant"
	"bs-notify-hub/internal/dispatch"
	"bs-notify-hub/internal/model"
	"fmt"

	"gorm.io/gorm"
)

type NotifyUnreadDBRepo struct {
	db *gorm.DB
}

func NewNotifyUnreadDBRepo(db *gorm.DB) *NotifyUnreadDBRepo {
	return &NotifyUnreadDBRepo{db: db}
}

// CountUnreadUser 统计【用户个人通知】未读数
func (r *NotifyUnreadDBRepo) CountUnreadUser(tenantID, userID string) (int64, error) {
	var count int64
	err := r.db.Model(&model.NotifyRecord{}).
		Joins("LEFT JOIN notify_status ON notify_record.notify_id = notify_status.notify_id AND notify_status.target_user_id = ?", userID).
		Where("notify_record.tenant_id = ?", tenantID).
		Where("notify_record.sender_type = ?", constant.SenderTypeUser).
		Where("notify_record.target_type != ?", dispatch.TargetBroadcast).
		Where("notify_status.status = ?", constant.NotifyStatusUnread).
		Count(&count).Error
	return count, err
}

// CountUnreadTenant 统计租户非广播未读数
func (r *NotifyUnreadDBRepo) CountUnreadTenant(tenantID, userID string) (int64, error) {
	var count int64
	err := r.db.Model(&model.NotifyRecord{}).
		Joins("LEFT JOIN notify_status ON notify_record.notify_id = notify_status.notify_id AND notify_status.target_user_id = ?", userID).
		Where("notify_record.tenant_id = ?", tenantID).
		Where("notify_record.sender_type = ?", constant.SenderTypeTenant).
		Where("notify_record.target_type != ?", dispatch.TargetBroadcast).
		Where("notify_status.status = ?", constant.NotifyStatusUnread).
		Count(&count).Error
	return count, err
}

// GetBroadcastOpIDs 获取用户在水位线之后针对广播的已操作 UserID
func (r *NotifyUnreadDBRepo) GetBroadcastOpIDs(tenantID, userID string) ([]string, error) {
	var opIDs []string

	err := r.db.Model(&model.NotifyStatus{}).
		Select("notify_status.notify_id").
		Joins("JOIN notify_record ON notify_record.notify_id = notify_status.notify_id").
		Where("notify_status.target_user_id = ?", userID).
		Where("notify_record.tenant_id = ?", tenantID).
		Where("notify_record.target_type = ?", dispatch.TargetBroadcast).
		Where("notify_status.status IN (?, ?)", constant.NotifyStatusRead, constant.NotifyStatusDeleted).
		Pluck("notify_status.notify_id", &opIDs).Error
	return opIDs, err
}

// CountBroadcastTotalByTenant 统计该租户下所有物理存在的广播 UserID 集合
func (r *NotifyUnreadDBRepo) CountBroadcastTotalByTenant(tenantID string) ([]string, error) {
	var totalIDs []string
	err := r.db.Model(&model.NotifyRecord{}).
		Select("notify_id").
		Where("tenant_id = ? AND target_type = ?", tenantID, dispatch.TargetBroadcast).
		Pluck("notify_id", &totalIDs).Error
	return totalIDs, err
}

// UserUnreadCount 对应用户不同类型通知的未读数(非广播)
type UserUnreadCount struct {
	UserID int64
	Count  int64
}

// BatchCountUnreadUser 批量统计多个用户的个人未读数
func (r *NotifyUnreadDBRepo) BatchCountUnreadUser(tenantID string, userIDs []string) (map[string]int64, error) {
	var results []UserUnreadCount
	err := r.db.Model(&model.NotifyRecord{}).
		Select("notify_status.target_user_id as user_id, COUNT(*) as count").
		Joins("JOIN notify_status ON notify_record.notify_id = notify_status.notify_id").
		Where("notify_record.tenant_id = ?", tenantID).
		Where("notify_status.target_user_id IN ?", userIDs).
		Where("notify_record.sender_type = ?", constant.SenderTypeUser).
		Where("notify_record.target_type != ?", dispatch.TargetBroadcast).
		Where("notify_status.status = ?", constant.NotifyStatusUnread).
		Group("notify_status.target_user_id").
		Scan(&results).Error

	resMap := make(map[string]int64)
	for _, item := range results {
		resMap[fmt.Sprintf("%d", item.UserID)] = item.Count
	}
	return resMap, err
}

// BatchCountUnreadTenant 批量统计多个用户的租户非广播未读数
func (r *NotifyUnreadDBRepo) BatchCountUnreadTenant(tenantID string, userIDs []string) (map[string]int64, error) {
	var results []UserUnreadCount
	err := r.db.Model(&model.NotifyRecord{}).
		Select("notify_status.target_user_id as user_id, COUNT(*) as count").
		Joins("JOIN notify_status ON notify_record.notify_id = notify_status.notify_id").
		Where("notify_record.tenant_id = ?", tenantID).
		Where("notify_status.target_user_id IN ?", userIDs).
		Where("notify_record.sender_type = ?", constant.SenderTypeTenant).
		Where("notify_record.target_type != ?", dispatch.TargetBroadcast).
		Where("notify_status.status = ?", constant.NotifyStatusUnread).
		Group("notify_status.target_user_id").
		Scan(&results).Error

	resMap := make(map[string]int64)
	for _, item := range results {
		resMap[fmt.Sprintf("%d", item.UserID)] = item.Count
	}
	return resMap, err
}
