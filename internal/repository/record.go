package repository

import (
	"bs-notify-hub/internal/constant"
	"bs-notify-hub/internal/dispatch"
	"bs-notify-hub/internal/model"
	"time"

	"github.com/google/uuid"
	"gorm.io/datatypes"

	"gorm.io/gorm"
)

type NotifyRecordRepo struct {
	db *gorm.DB
}

func NewNotifyRecordRepo(db *gorm.DB) *NotifyRecordRepo {
	return &NotifyRecordRepo{db: db}
}
func (r *NotifyRecordRepo) WithTx(db *gorm.DB) *NotifyRecordRepo {
	return &NotifyRecordRepo{db: db}
}

func (r *NotifyRecordRepo) Create(record *model.NotifyRecord) error {
	return r.db.Create(record).Error
}

func (r *NotifyRecordRepo) FindByID(notifyID string) (*model.NotifyRecord, error) {
	var record model.NotifyRecord
	err := r.db.Where("notify_id = ?", notifyID).First(&record).Error
	if err != nil {
		return nil, err
	}
	return &record, nil
}

func (r *NotifyRecordRepo) FindByTenant(tenantID string, limit, offset int) ([]model.NotifyRecord, error) {
	var records []model.NotifyRecord
	err := r.db.Where("tenant_id = ?", tenantID).Order("created_at DESC").Limit(limit).Offset(offset).Find(&records).Error
	return records, err
}

func (r *NotifyRecordRepo) Update(record *model.NotifyRecord) error {
	return r.db.Save(record).Error
}

func (r *NotifyRecordRepo) Delete(notifyID string) error {
	return r.db.Where("notify_id = ?", notifyID).Delete(&model.NotifyRecord{}).Error
}

type ExpiredNotifyInfo struct {
	NotifyID   uuid.UUID                   `gorm:"column:notify_id"`
	TenantID   string                      `gorm:"column:tenant_id"`
	TargetIDs  datatypes.JSONSlice[string] `gorm:"column:target_ids"`
	TargetType int8                        `gorm:"column:target_type"`
	SenderType int8                        `gorm:"column:sender_type"`
}

func (r *NotifyRecordRepo) ListExpiredNotifyInfos(before time.Time, limit int) ([]ExpiredNotifyInfo, error) {
	var results []ExpiredNotifyInfo

	err := r.db.Model(&model.NotifyRecord{}).
		Select("notify_id", "tenant_id", "target_ids", "target_type", "sender_type").
		Where("expire_at IS NOT NULL AND expire_at <= ?", before).
		Order("expire_at ASC").
		Limit(limit).
		Find(&results).Error

	return results, err
}

func (r *NotifyRecordRepo) DeleteByNotifyIDs(ids []uuid.UUID) error {
	if len(ids) == 0 {
		return nil
	}
	return r.db.Where("notify_id IN ?", ids).Delete(&model.NotifyRecord{}).Error
}

type RecordPageQueryRes struct {
	Total      int64
	PageIndex  int
	PageSize   int
	PagesTotal int
	List       []RecordHasStatus
}
type RecordHasStatus struct {
	NotifyID   string     `gorm:"column:notify_id" json:"notifyId"`
	Title      string     `gorm:"column:title" json:"title"`
	Content    string     `gorm:"column:content" json:"content"`
	SenderType int8       `gorm:"column:sender_type" json:"senderType"`
	SenderID   string     `gorm:"column:sender_id" json:"senderId"`
	EventType  string     `gorm:"column:event_type" json:"eventType"`
	ExpireAt   *time.Time `gorm:"column:expire_at" json:"expireAt"`
	CreatedAt  time.Time  `gorm:"column:created_at" json:"createdAt"`
	Status     int8       `gorm:"column:status"`
}

func (r *NotifyRecordRepo) GetPersonalPage(tenantID, userID string, pageIndex int, pageSize int) (RecordPageQueryRes, int64, error) {
	if pageIndex < 1 {
		pageIndex = 1
	}
	if pageSize < 1 {
		pageSize = 10
	}
	countRes := struct {
		Total  int64 `gorm:"column:total_cnt"`
		Unread int64 `gorm:"column:unread_cnt"`
	}{}
	var items []RecordHasStatus
	baseQuery := r.db.Model(&model.NotifyRecord{}).
		Joins("LEFT JOIN notify_status ON notify_record.notify_id = notify_status.notify_id AND notify_status.target_user_id = ?", userID).
		Where("notify_record.tenant_id = ?", tenantID).
		Where("notify_record.sender_type = ?", constant.SenderTypeUser).
		Where("(notify_status.status IS NULL OR notify_status.status != ?)", constant.NotifyStatusDeleted).
		Where("(notify_status.status IS NOT NULL OR notify_record.target_type = ?)", dispatch.TargetBroadcast)

	err := baseQuery.Session(&gorm.Session{}).
		Select("COUNT(*) AS total_cnt, SUM(CASE WHEN notify_status.status = ? OR notify_status.status IS NULL THEN 1 ELSE 0 END) AS unread_cnt", constant.NotifyStatusUnread).
		Scan(&countRes).Error

	if err != nil {
		return RecordPageQueryRes{}, 0, err
	}
	err = baseQuery.Session(&gorm.Session{}).
		Select([]string{
			"notify_record.notify_id",
			"notify_record.title",
			"notify_record.content",
			"notify_record.sender_type",
			"notify_record.sender_id",
			"notify_record.event_type",
			"notify_record.expire_at",
			"notify_record.created_at",
			"notify_status.status",
		}).
		Order("notify_record.created_at DESC").
		Offset((pageIndex - 1) * pageSize).
		Limit(pageSize).
		Scan(&items).Error
	if err != nil {
		return RecordPageQueryRes{}, 0, err
	}
	if items == nil {
		items = []RecordHasStatus{}
	}
	pagesTotal := 0
	if countRes.Total > 0 {
		pagesTotal = int((countRes.Total + int64(pageSize) - 1) / int64(pageSize))
	}
	return RecordPageQueryRes{
		Total:      countRes.Total,
		PageIndex:  pageIndex,
		PageSize:   pageSize,
		PagesTotal: pagesTotal,
		List:       items,
	}, countRes.Unread, nil
}

// GetPersonalStats 获取个人通知统计（总数与未读数），不查询列表
func (r *NotifyRecordRepo) GetPersonalStats(tenantID, userID string) (int64, int64, error) {
	countRes := struct {
		Total  int64 `gorm:"column:total_cnt"`
		Unread int64 `gorm:"column:unread_cnt"`
	}{}
	err := r.db.Model(&model.NotifyRecord{}).
		Joins("LEFT JOIN notify_status ON notify_record.notify_id = notify_status.notify_id AND notify_status.target_user_id = ?", userID).
		Where("notify_record.tenant_id = ?", tenantID).
		Where("notify_record.sender_type = ?", constant.SenderTypeUser).
		Where("(notify_status.status IS NULL OR notify_status.status != ?)", constant.NotifyStatusDeleted).
		Where("(notify_status.status IS NOT NULL OR notify_record.target_type = ?)", dispatch.TargetBroadcast).
		Select("COUNT(*) AS total_cnt, SUM(CASE WHEN notify_status.status = ? OR notify_status.status IS NULL THEN 1 ELSE 0 END) AS unread_cnt", constant.NotifyStatusUnread).
		Scan(&countRes).Error

	if err != nil {
		return 0, 0, err
	}
	return countRes.Total, countRes.Unread, nil
}

// GetTenantStats 获取租户通知统计（总数与未读数），不查询列表
func (r *NotifyRecordRepo) GetTenantStats(tenantID, userID string, watermark *model.NotifyWatermark) (int64, int64, error) {
	var clearAt, readAt time.Time
	if watermark != nil {
		clearAt = watermark.TenantClearAt
		readAt = watermark.TenantReadAt
	}
	if readAt.IsZero() {
		readAt = time.Date(1, 1, 1, 0, 0, 0, 0, time.UTC)
	}
	if clearAt.IsZero() {
		clearAt = time.Time{}
	}
	countRes := struct {
		Total  int64 `gorm:"column:total_cnt"`
		Unread int64 `gorm:"column:unread_cnt"`
	}{}

	err := r.db.Model(&model.NotifyRecord{}).
		Joins("LEFT JOIN notify_status ON notify_record.notify_id = notify_status.notify_id AND notify_status.target_user_id = ?", userID).
		Where("notify_record.tenant_id = ?", tenantID).
		Where("notify_record.sender_type = ?", constant.SenderTypeTenant).
		Where("notify_record.created_at > ?", clearAt).
		Where("(notify_status.status IS NULL OR notify_status.status != ?)", constant.NotifyStatusDeleted).
		Where("(notify_status.status IS NOT NULL OR notify_record.target_type = ?)", dispatch.TargetBroadcast).
		Select(`
    COUNT(*) AS total_cnt,
    SUM(
        CASE
            WHEN (notify_status.status IS NULL OR notify_status.status != ?)
                 AND notify_record.created_at > ?
            THEN 1
            ELSE 0
        END
    ) AS unread_cnt
`,
			constant.NotifyStatusRead,
			readAt,
		).
		Scan(&countRes).Error

	if err != nil {
		return 0, 0, err
	}
	return countRes.Total, countRes.Unread, nil
}

// GetTenantPage 获取租户类收件箱分页数据
// 基于水位线先过滤删除掉的记录，然后统计剩余记录，统计未读时判断状态不为已读且小于一键已读水位线的值
func (r *NotifyRecordRepo) GetTenantPage(tenantID, userID string, pageIndex, pageSize int, watermark *model.NotifyWatermark) (RecordPageQueryRes, int64, error) {
	if pageIndex < 1 {
		pageIndex = 1
	}
	if pageSize < 1 {
		pageSize = 10
	}

	var clearAt, readAt time.Time
	if watermark != nil {
		clearAt = watermark.TenantClearAt
		readAt = watermark.TenantReadAt
	} else {
		clearAt = time.Time{}
		readAt = time.Time{}
	}

	countRes := struct {
		Total  int64 `gorm:"column:total_cnt"`
		Unread int64 `gorm:"column:unread_cnt"`
	}{}
	var items []RecordHasStatus

	baseQuery := r.db.Model(&model.NotifyRecord{}).
		Joins("LEFT JOIN notify_status ON notify_record.notify_id = notify_status.notify_id AND notify_status.target_user_id = ?", userID).
		Where("notify_record.tenant_id = ?", tenantID).
		Where("notify_record.sender_type = ?", constant.SenderTypeTenant).
		Where("notify_record.created_at > ?", clearAt).
		Where("(notify_status.status IS NULL OR notify_status.status != ?)", constant.NotifyStatusDeleted).
		Where("(notify_status.status IS NOT NULL OR notify_record.target_type = ?)", dispatch.TargetBroadcast)

	err := baseQuery.Session(&gorm.Session{}).
		Select(`
    COUNT(*) AS total_cnt,
    SUM(
        CASE
            WHEN (notify_status.status IS NULL OR notify_status.status != ?)
                 AND notify_record.created_at > ?
            THEN 1
            ELSE 0
        END
    ) AS unread_cnt
`,
			constant.NotifyStatusRead,
			readAt,
		).
		Scan(&countRes).Error

	if err != nil {
		return RecordPageQueryRes{}, 0, err
	}

	// 修复点 2：合并两个 Select，将 gorm.Expr 作为多参数之一传入，防止覆盖和解构失败
	err = baseQuery.Session(&gorm.Session{}).
		Select(`
        notify_record.notify_id,
        notify_record.title,
        notify_record.content,
        notify_record.sender_type,
        notify_record.sender_id,
        notify_record.event_type,
        notify_record.expire_at,
        notify_record.created_at,
        CASE
            WHEN notify_status.status IS NOT NULL
                THEN notify_status.status
            WHEN notify_record.created_at <= ?
                THEN ?
            ELSE ?
        END AS status
    `,
			readAt,
			constant.NotifyStatusRead,
			constant.NotifyStatusUnread,
		).
		Order("notify_record.created_at DESC").
		Offset((pageIndex - 1) * pageSize).
		Limit(pageSize).
		Scan(&items).Error

	if err != nil {
		return RecordPageQueryRes{}, 0, err
	}

	if items == nil {
		items = []RecordHasStatus{}
	}

	pagesTotal := 0
	if countRes.Total > 0 {
		pagesTotal = int((countRes.Total + int64(pageSize) - 1) / int64(pageSize))
	}

	return RecordPageQueryRes{
		Total:      countRes.Total,
		PageIndex:  pageIndex,
		PageSize:   pageSize,
		PagesTotal: pagesTotal,
		List:       items,
	}, countRes.Unread, nil
}
