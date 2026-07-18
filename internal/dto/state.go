package dto

// OperateNotifyStatusReq 已读/删除通知请求
type OperateNotifyStatusReq struct {
	NotifyID string `json:"notifyId" vd:"$!='';msg:'消息Id必填'"`
	UserID   string `json:"userId" vd:"$!='';msg:'用户Id必填'"`
	TenantID string `json:"tenantId" vd:"$!='';msg:'租户Id必填'"`
}

// OperateNotifyStatusRes 已读/删除通知响应
type OperateNotifyStatusRes struct {
	NotifyID string `json:"notifyId"`
	Status   int8   `json:"status"`
}

// BatchOperateNotifyStatusReq 一键状态操作请求 (已读/清空)
type BatchOperateNotifyStatusReq struct {
	UserID   string `json:"userId" vd:"$!='';msg:'用户Id必填'"`
	TenantID string `json:"tenantId" vd:"$!='';msg:'租户Id必填'"`
	Type     int8   `json:"type" vd:"$==0||$==1;msg:'类型应为 0-用户 或 1-租户'"` // 0-用户, 1-租户
}
