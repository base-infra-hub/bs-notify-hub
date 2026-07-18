package service

import (
	"bs-notify-hub/internal/dto"
	"bs-notify-hub/internal/repository"
	"bs-notify-hub/pkg/response"
	"context"
	"sync"
	"time"

	"bs-notify-hub/api/proto"

	"github.com/cloudwego/hertz/pkg/common/hlog"
	"google.golang.org/protobuf/types/known/emptypb"
)

// GRPCNotifyService gRPC 通知服务适配层
type GRPCNotifyService struct {
	proto.UnimplementedNotifyServiceServer
	senderSvc *NotifySenderService
	statusSvc *NotifyStatusService
	ticketSvc *TicketService
	inboxSvc  *NotifyInboxService
}

var (
	grpcNotifyInstance *GRPCNotifyService
	grpcNotifyOnce     sync.Once
)

// GetGRPCNotifyService 获取 gRPC 通知服务单例
func GetGRPCNotifyService() *GRPCNotifyService {
	grpcNotifyOnce.Do(func() {
		grpcNotifyInstance = &GRPCNotifyService{
			senderSvc: GetNotifySenderService(),
			statusSvc: GetNotifyStatusService(),
			ticketSvc: GetTicketService(),
			inboxSvc:  GetNotifyInboxService(),
		}
	})
	return grpcNotifyInstance
}

func (s *GRPCNotifyService) SendToUser(ctx context.Context, req *proto.SendToUserReq) (*proto.SendNotifyRes, error) {
	hlog.Infof("[gRPC] SendToUser 调用开始, tenantID:%s, userID:%s", req.TenantId, req.UserId)
	prop := SendNotifyProp{
		Title:      req.Title,
		Content:    req.Content,
		TenantID:   req.TenantId,
		SenderID:   req.SenderId,
		SenderType: toSenderTypeCode(req.SenderType),
		EventType:  req.EventType,
		TTLSeconds: req.TtlSeconds,
		TargetIDs:  []string{req.UserId},
	}
	result, codeErr := s.senderSvc.SendToUser(ctx, prop)
	if codeErr != nil {
		hlog.Errorf("[gRPC] SendToUser 调用失败, tenantID:%s, userID:%s, err:%v", req.TenantId, req.UserId, codeErr)
		return nil, codeErr
	}
	res := &proto.SendNotifyRes{
		NotifyId:   result.NotifyID,
		TtlSeconds: result.TTLSeconds,
		ExpireTime: func() *string {
			if result.ExpireTime != nil {
				tStr := result.ExpireTime.Format(time.RFC3339)
				return &tStr
			}
			return nil
		}(),
	}
	return res, nil
}

// send

func (s *GRPCNotifyService) SendToUsers(ctx context.Context, req *proto.SendToUsersReq) (*proto.SendNotifyRes, error) {
	hlog.Infof("[gRPC] SendToUsers 调用开始, tenantID:%s, userIDs:%v", req.TenantId, req.UserIds)
	prop := SendNotifyProp{
		Title:      req.Title,
		Content:    req.Content,
		TenantID:   req.TenantId,
		SenderID:   req.SenderId,
		SenderType: toSenderTypeCode(req.SenderType),
		EventType:  req.EventType,
		TTLSeconds: req.TtlSeconds,
		TargetIDs:  req.UserIds,
	}
	result, codeErr := s.senderSvc.SendToUsers(ctx, prop)
	if codeErr != nil {
		hlog.Errorf("[gRPC] SendToUsers 调用失败, tenantID:%s, userIDs:%v, err:%v", req.TenantId, req.UserIds, codeErr)
		return nil, codeErr
	}
	res := &proto.SendNotifyRes{
		NotifyId:   result.NotifyID,
		TtlSeconds: result.TTLSeconds,
		ExpireTime: func() *string {
			if result.ExpireTime != nil {
				tStr := result.ExpireTime.Format(time.RFC3339)
				return &tStr
			}
			return nil
		}(),
	}
	return res, nil
}

func (s *GRPCNotifyService) SendToAll(ctx context.Context, req *proto.SendToAllReq) (*proto.SendNotifyRes, error) {
	hlog.Infof("[gRPC] SendToAll 调用开始, tenantID:%s", req.TenantId)
	prop := SendNotifyProp{
		Title:      req.Title,
		Content:    req.Content,
		TenantID:   req.TenantId,
		SenderID:   req.SenderId,
		SenderType: toSenderTypeCode(req.SenderType),
		EventType:  req.EventType,
		TTLSeconds: req.TtlSeconds,
		TargetIDs:  nil,
	}
	result, codeErr := s.senderSvc.SendToAll(ctx, prop)
	if codeErr != nil {
		hlog.Errorf("[gRPC] SendToAll 调用失败, tenantID:%s, err:%v", req.TenantId, codeErr)
		return nil, codeErr
	}
	res := &proto.SendNotifyRes{
		NotifyId:   result.NotifyID,
		TtlSeconds: result.TTLSeconds,
		ExpireTime: func() *string {
			if result.ExpireTime != nil {
				tStr := result.ExpireTime.Format(time.RFC3339)
				return &tStr
			}
			return nil
		}(),
	}
	return res, nil
}

// status

func (s *GRPCNotifyService) MarkRead(ctx context.Context, req *proto.OperateNotifyStatusReq) (*emptypb.Empty, error) {
	hlog.Infof("[gRPC] MarkRead 调用开始, notifyID:%s, userID:%s, tenantID:%s", req.NotifyId, req.UserId, req.TenantId)
	codeErr := s.statusSvc.MarkRead(ctx, req.NotifyId, req.UserId, req.TenantId)
	if codeErr != nil {
		hlog.Errorf("[gRPC] MarkRead 调用失败, notifyID:%s, userID:%s, tenantID:%s, err:%v", req.NotifyId, req.UserId, req.TenantId, codeErr)
		return nil, codeErr
	}
	return &emptypb.Empty{}, nil
}

func (s *GRPCNotifyService) DeleteNotify(ctx context.Context, req *proto.OperateNotifyStatusReq) (*emptypb.Empty, error) {
	hlog.Infof("[gRPC] DeleteNotify 调用开始, notifyID:%s, userID:%s, tenantID:%s", req.NotifyId, req.UserId, req.TenantId)
	codeErr := s.statusSvc.DeleteNotify(ctx, req.NotifyId, req.UserId, req.TenantId)
	if codeErr != nil {
		hlog.Errorf("[gRPC] DeleteNotify 调用失败, notifyID:%s, userID:%s, tenantID:%s, err:%v", req.NotifyId, req.UserId, req.TenantId, codeErr)
		return nil, codeErr
	}
	return &emptypb.Empty{}, nil
}

// applyTicket

func (s *GRPCNotifyService) ApplyTicket(ctx context.Context, req *proto.ApplyTicketReq) (*proto.ApplyTicketRes, error) {
	hlog.Infof("[gRPC] ApplyTicket 调用开始, tenant:%s, userID:%s", req.Tenant, req.UserId)
	tkt, exp, cre, codeErr := s.ticketSvc.ApplyTicket(ctx, req.Tenant, req.UserId)
	if codeErr != nil {
		hlog.Errorf("[gRPC] ApplyTicket 调用失败, tenant:%s, userID:%s, err:%v", req.Tenant, req.UserId, codeErr)
		return nil, codeErr
	}
	return &proto.ApplyTicketRes{Ticket: tkt, ExpireTime: exp.Format(time.RFC3339), CreateTime: cre.Format(time.RFC3339)}, nil
}

// batch status

func (s *GRPCNotifyService) BatchMarkRead(ctx context.Context, req *proto.BatchOperateNotifyStatusReq) (*emptypb.Empty, error) {
	hlog.Infof("[gRPC] BatchMarkRead 调用开始, userID:%s, tenantID:%s, type:%d", req.UserId, req.TenantId, req.Type)
	var codeErr *response.CodeError
	if req.Type == int32(proto.SenderType_TENANT) {
		codeErr = s.statusSvc.TenantBatchMarkRead(ctx, req.UserId, req.TenantId)
	} else {
		codeErr = s.statusSvc.UserBatchMarkRead(ctx, req.UserId, req.TenantId)
	}
	if codeErr != nil {
		hlog.Errorf("[gRPC] BatchMarkRead 调用失败, userID:%s, tenantID:%s, type:%d, err:%v", req.UserId, req.TenantId, req.Type, codeErr)
		return nil, codeErr
	}
	return &emptypb.Empty{}, nil
}

func (s *GRPCNotifyService) BatchDeleteNotify(ctx context.Context, req *proto.BatchOperateNotifyStatusReq) (*emptypb.Empty, error) {
	hlog.Infof("[gRPC] BatchDeleteNotify 调用开始, userID:%s, tenantID:%s, type:%d", req.UserId, req.TenantId, req.Type)
	var codeErr *response.CodeError
	if req.Type == int32(proto.SenderType_TENANT) {
		codeErr = s.statusSvc.TenantBatchDelete(ctx, req.UserId, req.TenantId)
	} else {
		codeErr = s.statusSvc.UserBatchDelete(ctx, req.UserId, req.TenantId)
	}
	if codeErr != nil {
		hlog.Errorf("[gRPC] BatchDeleteNotify 调用失败, userID:%s, tenantID:%s, type:%d, err:%v", req.UserId, req.TenantId, req.Type, codeErr)
		return nil, codeErr
	}
	return &emptypb.Empty{}, nil
}

// page

// GetPersonalInbox 获取个人收件箱分页数据
func (s *GRPCNotifyService) GetPersonalInbox(ctx context.Context, req *proto.GetInboxReq) (*proto.GetInboxRes, error) {
	hlog.Infof("[gRPC] GetPersonalInbox 调用开始, tenantID:%s, userID:%s, pageIndex:%d, pageSize:%d", req.TenantId, req.UserId, req.PageIndex, req.PageSize)
	res, unread, err := s.inboxSvc.GetPersonalPageRPC(ctx, &dto.InboxPageReq{
		TenantID:  req.TenantId,
		UserID:    req.UserId,
		PageIndex: int(req.PageIndex),
		PageSize:  int(req.PageSize),
	})
	if err != nil {
		hlog.Errorf("[gRPC] GetPersonalInbox 调用失败, tenantID:%s, userID:%s, err:%v", req.TenantId, req.UserId, err)
		return nil, err
	}
	return s.toProtoInboxRes(res, unread), nil
}

// GetTenantInbox 查询租户收件箱分页数据
func (s *GRPCNotifyService) GetTenantInbox(ctx context.Context, req *proto.GetInboxReq) (*proto.GetInboxRes, error) {
	hlog.Infof("[gRPC] GetTenantInbox 调用开始, tenantID:%s, userID:%s, pageIndex:%d, pageSize:%d", req.TenantId, req.UserId, req.PageIndex, req.PageSize)
	res, unread, err := s.inboxSvc.GetTenantPageRPC(ctx, &dto.InboxPageReq{
		TenantID:  req.TenantId,
		UserID:    req.UserId,
		PageIndex: int(req.PageIndex),
		PageSize:  int(req.PageSize),
	})
	if err != nil {
		hlog.Errorf("[gRPC] GetTenantInbox 调用失败, tenantID:%s, userID:%s, err:%v", req.TenantId, req.UserId, err)
		return nil, err
	}

	return s.toProtoInboxRes(res, unread), nil
}

// toProtoInboxRes 将底层通用的分页结果转换为 RPC 协议定义的结构
func (s *GRPCNotifyService) toProtoInboxRes(recordPageQueryRes repository.RecordPageQueryRes, unread int64) *proto.GetInboxRes {
	items := make([]*proto.InboxItem, len(recordPageQueryRes.List))
	for i, item := range recordPageQueryRes.List {
		items[i] = &proto.InboxItem{
			NotifyId:   item.NotifyID,
			Title:      item.Title,
			Content:    item.Content,
			SenderType: int32(item.SenderType),
			SenderId:   item.SenderID,
			EventType:  item.EventType,
			Status:     int32(item.Status),
			ExpireAt: func() *string {
				if item.ExpireAt != nil {
					tStr := item.ExpireAt.Format(time.RFC3339)
					return &tStr
				}
				return nil
			}(),
			CreatedAt: item.CreatedAt.Format(time.RFC3339),
		}
	}

	return &proto.GetInboxRes{
		PageIndex: int32(recordPageQueryRes.PageIndex),
		PageSize:  int32(recordPageQueryRes.PageSize),
		Total:     recordPageQueryRes.Total,
		Unread:    unread,
		List:      items,
	}
}

// GetUserNotifyStats 获取用户个人和租户通知统计（总数与未读数）
func (s *GRPCNotifyService) GetUserNotifyStats(ctx context.Context, req *proto.GetUserNotifyStatsReq) (*proto.GetUserNotifyStatsRes, error) {
	hlog.Infof("[gRPC] GetUserNotifyStats 调用开始, tenantID:%s, userID:%s", req.TenantId, req.UserId)
	pTotal, pUnread, err := s.inboxSvc.GetPersonalStats(ctx, req.TenantId, req.UserId)
	if err != nil {
		hlog.Errorf("[gRPC] GetUserNotifyStats 获取个人统计失败, tenantID:%s, userID:%s, err:%v", req.TenantId, req.UserId, err)
		return nil, err
	}
	tTotal, tUnread, err := s.inboxSvc.GetTenantStats(ctx, req.TenantId, req.UserId)
	if err != nil {
		hlog.Errorf("[gRPC] GetUserNotifyStats 获取租户统计失败, tenantID:%s, userID:%s, err:%v", req.TenantId, req.UserId, err)
		return nil, err
	}
	return &proto.GetUserNotifyStatsRes{
		Personal: &proto.NotifyStats{Total: pTotal, Unread: pUnread},
		Tenant:   &proto.NotifyStats{Total: tTotal, Unread: tUnread},
	}, nil
}

// Connect 测试远端调用连通性
func (s *GRPCNotifyService) Connect(ctx context.Context, req *emptypb.Empty) (*proto.ConnectRes, error) {
	hlog.Infof("[gRPC] Connect 调用开始")
	return &proto.ConnectRes{Message: "成功"}, nil
}

func toSenderTypeCode(senderType proto.SenderType) int8 {
	return int8(senderType)
}
