package dto

import "time"

// InboxPageReq 收件箱分页查询请求
type InboxPageReq struct {
	TenantID  string `json:"tenantId" vd:"$!='';msg:'租户Id必填'"`
	UserID    string `json:"userId" vd:"$!='';msg:'userId必填'"`
	PageIndex int    `json:"pageIndex" vd:"$>0;msg:'页码必须大于0'"`
	PageSize  int    `json:"pageSize" vd:"$>0&&$<=100;msg:'每页数量必须在1-100之间'"`
}

type InboxPageRes struct {
	Total      int64       `json:"total"`
	PageIndex  int         `json:"pageIndex"`
	PageSize   int         `json:"pageSize"`
	PagesTotal int         `json:"pagesTotal"`
	Unread     int64       `json:"unread"`
	List       []InboxItem `json:"list"`
}
type InboxItem struct {
	NotifyID   string     `json:"notifyId"`
	Title      string     `json:"title"`
	Content    string     `json:"content"`
	SenderType int8       `json:"senderType"`
	SenderID   string     `json:"senderId"`
	EventType  string     `json:"eventType"`
	ExpireAt   *time.Time `json:"expireAt"`
	CreatedAt  time.Time  `json:"createdAt"`
	Status     int8       `json:"status"`
}
