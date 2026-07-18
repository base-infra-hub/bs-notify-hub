package dto

import "time"

type SubscribeParam struct {
	UserID   string
	TenantID string
	ConnID   string
}
type MsgType string

const (
	NotifyMsg         MsgType = "NOTIFY"
	TenantUnreadMsg   MsgType = "TENANT_UNREAD"
	PersonalUnreadMsg MsgType = "PERSONAL_UNREAD"
	Heartbeat         MsgType = "HEARTBEAT"
)

type InternalMsg struct {
	Type MsgType
	Data interface{}
}
type NotifyMsgData struct {
	NotifyID   string     `json:"notifyId"`
	Title      string     `json:"title"`
	Content    string     `json:"content"`
	TenantID   string     `json:"tenantId"`
	SenderID   string     `json:"senderId"`
	SenderType int8       `json:"senderType"`
	TargetIDs  []string   `json:"targetIds"`
	EventType  string     `json:"eventType"`
	ExpireTime *time.Time `json:"expireTime"`
}

// ApplyTicketReq 凭证申请请求
type ApplyTicketReq struct {
	Tenant string `json:"tenant" vd:"$!='';msg:'租户标识必填'"`
	UserID string `json:"userId" vd:"$!='';msg:'用户标识必填'"`
}

// ApplyTicketRes 凭证申请响应
type ApplyTicketRes struct {
	Ticket     string    `json:"ticket"`
	ExpireTime time.Time `json:"expireTime"`
	CreateTime time.Time `json:"createTime"`
}
