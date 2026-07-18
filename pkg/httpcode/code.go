package httpcode

type HttpCode int

// 业务错误码定义
// msg 携带具体的错误详情；code 只描述错误的类别。
const (
	Success         HttpCode = 0   // 成功
	BadRequest      HttpCode = 400 // 错误请求（参数无效、缺少字段等）
	Unauthorized    HttpCode = 401 // 未授权 / 凭证无效
	Forbidden       HttpCode = 403 // 权限不足（已登录但无权操作该租户/记录）
	NotFound        HttpCode = 404 // 资源未找到
	DBError         HttpCode = 500 // 数据库错误
	Conflict        HttpCode = 409 // 冲突（重名、非空删除等）
	RateLimitExceed HttpCode = 429 // 请求过于频繁（针对发送通知或标记已读的限流）
	InvalidID       HttpCode = 422 // UserID 格式错误（Base64 解码失败等）
	InternalError   HttpCode = 500 // 内部 / 数据库错误
	Timeout         HttpCode = 504 // 网关或下游服务超时
)
