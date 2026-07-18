package dto

import "time"

// BaseSendReq 基础发送请求通用字段
type BaseSendReq struct {
	Title      string `json:"title" vd:"$!='';msg:'标题必填'"`
	Content    string `json:"content" vd:"$!='';msg:'内容必填'"`
	TenantID   string `json:"tenantId" vd:"$!='';msg:'租户ID必填'"`
	SenderID   string `json:"senderId" vd:"$!='';msg:'发送者ID必填'"`
	SenderType int8   `json:"senderType" vd:"$!='';msg:'发送者类型必填'"`
	EventType  string `json:"eventType" vd:"$!='';msg:'事件类型必填'"`
	TTLSeconds *int64 `json:"ttlSeconds"`
}

// SendToUserReq 一对一发送请求
type SendToUserReq struct {
	BaseSendReq
	UserID string `json:"userId" vd:"$!='';msg:'目标用户Id必填'"`
}

// SendToUsersReq 一对多发送请求
type SendToUsersReq struct {
	BaseSendReq
	UserIDs []string `json:"userIds" vd:"len($)>0;msg:'目标用户列表不能为空'"`
}

// SendToAllReq 租户全员发送请求
type SendToAllReq struct {
	BaseSendReq
}

// NotifyRes 统一通知响应（原 NotificationRes）
type NotifyRes struct {
	NotifyID   string     `json:"notifyId"`
	ExpireTime *time.Time `json:"expireTime"`
	TTLSeconds int64      `json:"ttlSeconds"`
}
