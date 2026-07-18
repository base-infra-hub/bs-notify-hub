package service

import (
	"bs-notify-hub/internal/dto"
	"bs-notify-hub/internal/repository"
	"bs-notify-hub/pkg/db"
	"bs-notify-hub/pkg/httpcode"
	"bs-notify-hub/pkg/response"
	"context"
	"sync"

	"github.com/cloudwego/hertz/pkg/common/hlog"
	"gorm.io/gorm"
)

type NotifyInboxService struct {
	db            *gorm.DB
	recordRepo    *repository.NotifyRecordRepo
	watermarkRepo *repository.NotifyWatermarkRepo
}

var (
	notifyInboxInstance *NotifyInboxService
	notifyInboxOnce     sync.Once
)

func GetNotifyInboxService() *NotifyInboxService {
	appDB := db.GetDB()
	notifyInboxOnce.Do(func() {
		notifyInboxInstance = &NotifyInboxService{
			db:            appDB,
			recordRepo:    repository.NewNotifyRecordRepo(appDB),
			watermarkRepo: repository.NewNotifyWatermarkRepo(appDB),
		}
	})
	return notifyInboxInstance
}

// GetPersonalPageHttp 获取个人收件箱分页,提供给Http使用
func (s *NotifyInboxService) GetPersonalPageHttp(ctx context.Context, query *dto.InboxPageReq) (dto.InboxPageRes, *response.CodeError) {
	page, unread, err := s.recordRepo.GetPersonalPage(query.TenantID, query.UserID, query.PageIndex, query.PageSize)
	if err != nil {
		hlog.Errorf("个人通知查询失败: %v", err)
		return dto.InboxPageRes{}, response.NewCodeError(httpcode.DBError, "查询失败")
	}
	return s.convertToPageRes(page, unread), nil
}

// GetTenantPageHttp 获取租户收件箱分页,提供给Http使用
func (s *NotifyInboxService) GetTenantPageHttp(ctx context.Context, query *dto.InboxPageReq) (dto.InboxPageRes, *response.CodeError) {
	watermark, err := s.watermarkRepo.FindByTenantAndUser(query.TenantID, query.UserID)
	if err != nil {
		hlog.Errorf("获取用户通知水位线失败: %v", err)
		return dto.InboxPageRes{}, response.NewCodeError(httpcode.DBError, "查询失败")
	}
	page, unread, err := s.recordRepo.GetTenantPage(query.TenantID, query.UserID, query.PageIndex, query.PageSize, watermark)
	if err != nil {
		hlog.Errorf("租户通知查询失败: %v", err)
		return dto.InboxPageRes{}, response.NewCodeError(httpcode.DBError, "查询失败")
	}
	return s.convertToPageRes(page, unread), nil
}
func (s *NotifyInboxService) GetPersonalPageRPC(ctx context.Context, query *dto.InboxPageReq) (repository.RecordPageQueryRes, int64, error) {
	page, unread, err := s.recordRepo.GetPersonalPage(query.TenantID, query.UserID, query.PageIndex, query.PageSize)
	if err != nil {
		hlog.Errorf("个人通知查询失败: %v", err)
		return repository.RecordPageQueryRes{}, 0, response.NewCodeError(httpcode.DBError, "查询失败")
	}
	return page, unread, nil
}
func (s *NotifyInboxService) GetTenantPageRPC(ctx context.Context, query *dto.InboxPageReq) (repository.RecordPageQueryRes, int64, error) {
	watermark, err := s.watermarkRepo.FindByTenantAndUser(query.TenantID, query.UserID)
	if err != nil {
		hlog.Errorf("获取用户通知水位线失败: %v", err)
		return repository.RecordPageQueryRes{}, 0, response.NewCodeError(httpcode.DBError, "查询失败")
	}
	page, unread, err := s.recordRepo.GetTenantPage(query.TenantID, query.UserID, query.PageIndex, query.PageSize, watermark)
	if err != nil {
		hlog.Errorf("租户通知查询失败: %v", err)
		return repository.RecordPageQueryRes{}, 0, response.NewCodeError(httpcode.DBError, "查询失败")
	}
	return page, unread, nil
}

// GetPersonalStats 获取个人通知统计（总数与未读数）
func (s *NotifyInboxService) GetPersonalStats(ctx context.Context, tenantID, userID string) (int64, int64, error) {
	total, unread, err := s.recordRepo.GetPersonalStats(tenantID, userID)
	if err != nil {
		hlog.Errorf("个人通知统计查询失败: %v", err)
		return 0, 0, response.NewCodeError(httpcode.DBError, "查询失败")
	}
	return total, unread, nil
}

// GetTenantStats 获取租户通知统计（总数与未读数）
func (s *NotifyInboxService) GetTenantStats(ctx context.Context, tenantID, userID string) (int64, int64, error) {
	watermark, err := s.watermarkRepo.FindByTenantAndUser(tenantID, userID)
	if err != nil {
		hlog.Errorf("获取用户通知水位线失败: %v", err)
		return 0, 0, response.NewCodeError(httpcode.DBError, "查询失败")
	}
	total, unread, err := s.recordRepo.GetTenantStats(tenantID, userID, watermark)
	if err != nil {
		hlog.Errorf("租户通知统计查询失败: %v", err)
		return 0, 0, response.NewCodeError(httpcode.DBError, "查询失败")
	}
	return total, unread, nil
}

// 抽离通用转换逻辑 (内部私有方法)
func (s *NotifyInboxService) convertToPageRes(page repository.RecordPageQueryRes, unread int64) dto.InboxPageRes {
	resItems := make([]dto.InboxItem, len(page.List))
	for i, item := range page.List {
		resItems[i] = dto.InboxItem{
			NotifyID:   item.NotifyID,
			Title:      item.Title,
			Content:    item.Content,
			SenderType: item.SenderType,
			SenderID:   item.SenderID,
			EventType:  item.EventType,
			CreatedAt:  item.CreatedAt,
			ExpireAt:   item.ExpireAt,
			Status:     item.Status,
		}
	}
	return dto.InboxPageRes{
		Total:      page.Total,
		PageIndex:  page.PageIndex,
		PageSize:   page.PageSize,
		PagesTotal: page.PagesTotal,
		Unread:     unread,
		List:       resItems,
	}
}
