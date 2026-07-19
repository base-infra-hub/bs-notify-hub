# 接入整合开发文档

本服务对外提供 **HTTP RESTful API** 与高并发 **gRPC** 两种集成方式，适用于微服务集群及第三方业务系统投递与接收通知。

---

## 🔒 鉴权安全规范 (RSA JWT)

除了最终浏览器的 SSE 长连接外，所有的 HTTP 与 gRPC 请求均强制进行 **RS256 JWT** 安全验签。

### 1. 公钥声明与配置
在 `config.yaml` 中配置 `auth` 信息。服务端的公钥来源于你的授权中心服务，支持两种格式：
* **PEM 格式 (推荐)**：含 `-----BEGIN PUBLIC KEY-----` 头尾。
* **裸 Base64 格式**：直接粘贴公钥内容的 Base64 字符串，程序会自动在内存中构建为 PEM。

```yaml
auth:
  service_tag: "BS-Notify-Hub" # 服务安全标识 (JWT payload 中的 tag 必须与此一致)
  rsa_public_key: |
    -----BEGIN PUBLIC KEY-----
    MIIBIjANBgkqhkiG9w0BAQEFAAOCAQ8AMIIBCgKCAQEA...
    -----END PUBLIC KEY-----
```

### 2. JWT 签名与荷载 (Claims)
在你的授权中心或客户端签发 JWT 时：
* **Header**：`alg` 必须为 `RS256`（**不支持且会拒绝** RS384/RS512 或 HS256 等其他算法）。
* **Payload (荷载)**：
  * `tag` (String)：必须完全等于配置文件中的 `auth.service_tag`（用于防止 Token 被复用到其他不相关的服务上）。
  * `tenant_id` (String)：**必填。** 绑定第三方所归属的租户空间标识。服务端将强行以此标识做租户多数据隔离，禁止越权操作。
  * `exp` (Numeric/String/Null)：过期时间。如果 `exp` 缺失或为 `null`，本系统将判定此 Token **永不过期**。如果提供了具体时间戳，则严格校验是否已过期。

> [!TIP]
> **配套授权中心部署建议 (推荐)**：
> 如果你不想在业务系统里自己写代码或配置私钥签发 JWT，可以直接拉取并部署项目组配套的轻量级非对称密钥授权服务 **[bs-auth](https://github.com/base-infra-hub/bs-auth)**。
> 在 `bs-auth` 后台管理控制台上，你只需要为该客户端配置对应的服务 Tag（`BS-Notify-Hub`）和它所属的 `tenant_id`，即可一键生成永久有效或设定有效期的 RSA 签名 JWT Token，免去任何编码对接开发。

### 3. 自己签发 JWT 示例 (Self-issued)
如果你需要自己编写代码来生成对接 `bs-notify-hub` 的凭证，请务必使用 **RSA 密钥对** 以及 **RS256 算法** 进行签名。

#### 💡 Node.js 签发示例
```javascript
const jwt = require('jsonwebtoken');
const fs = require('fs');

// 读取与 config.yaml 中配置公钥相匹配的 RSA 私钥
const privateKey = fs.readFileSync('private.key');

const token = jwt.sign(
    { 
        tag: 'BS-Notify-Hub', // 必须与服务端的 auth.service_tag 一致
        tenant_id: 'tenantA',  // 👈 统一新接入规范：必须指定该 Token 允许接入的租户ID
        sub: 'your-client-id'
    }, 
    privateKey, 
    { 
        algorithm: 'RS256',  // 必须强制指定为 RS256
        expiresIn: '1h'       // 也可以不设 exp 荷载，代表永久有效
    }
);

console.log("Authorization: Bearer " + token);
```

#### 💡 Go 签发示例
```go
package main

import (
	"crypto/rsa"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

func GenerateToken(privKey *rsa.PrivateKey, serviceTag string, tenantID string) (string, error) {
	claims := jwt.MapClaims{
		"tag":       serviceTag,                    // 服务匹配标识
		"tenant_id": tenantID,                      // 👈 租户空间硬绑定，防跨租户伪造
		"sub":       "your-client-id",
		"iat":       time.Now().Unix(),
		// "exp": time.Now().Add(time.Hour).Unix(), // 注释掉或设为 null 即代表永不过期
	}

	// 必须使用 jwt.SigningMethodRS256
	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	return token.SignedString(privKey)
}
```

---

## 🌐 HTTP RESTful 接口

请求要求：
* **Header** 必须携带：`Authorization: Bearer <Your_JWT_Token>`
* 统一 API 前缀：`/v1`

### 1. 发送消息接口 (Sender)

#### ① 发送单用户通知
* **POST** `/v1/sender/user`
* **Payload (JSON)**:
```json
{
  "tenant_id": "tenantA",
  "user_id": "user123",
  "title": "系统维护通知",
  "content": "系统将在今晚进行维护升级，请做好备份。"
}
```

#### ② 批量发送指定用户
* **POST** `/v1/sender/users`
* **Payload (JSON)**:
```json
{
  "tenant_id": "tenantA",
  "user_ids": ["user123", "user456"],
  "title": "团队会议提醒",
  "content": "今天下午 3:00 在会议室 A 召开周会。"
}
```

#### ③ 广播发送（租户下全体在线用户）
* **POST** `/v1/sender/all`
* **Payload (JSON)**:
```json
{
  "tenant_id": "tenantA",
  "title": "全局紧急通知",
  "content": "网络出口节点异常，部分内网服务可能受影响。"
}
```

---

### 2. 收件箱数据查询 (Inbox)

#### ① 查询用户个人私信通知 (带分页)
* **POST** `/v1/inbox/personal`
* **Payload (JSON)**:
```json
{
  "tenant_id": "tenantA",
  "user_id": "user123",
  "page": 1,
  "page_size": 10
}
```

#### ② 查询租户发来的广播/组通知 (带分页)
* **POST** `/v1/inbox/tenant`
* **Payload (JSON)**:
```json
{
  "tenant_id": "tenantA",
  "user_id": "user123",
  "page": 1,
  "page_size": 10
}
```

---

### 3. 通知状态变更 (Status)

#### ① 标记单条通知已读
* **POST** `/v1/status/read`
* **Payload (JSON)**:
```json
{
  "tenant_id": "tenantA",
  "user_id": "user123",
  "notify_id": 998877,
  "category": 0   // 0: 个人私信, 1: 租户广播
}
```

#### ② 批量标记已读
* **POST** `/v1/status/read/all`
* **Payload (JSON)**:
```json
{
  "tenant_id": "tenantA",
  "user_id": "user123",
  "category": 0
}
```

#### ③ 删除单条通知
* **DELETE** `/v1/status/delete`
* **Payload (JSON)**:
```json
{
  "tenant_id": "tenantA",
  "user_id": "user123",
  "notify_id": 998877,
  "category": 0
}
```

---

## ⚡ SSE 实时订阅接入 (Server-Sent Events)

浏览器原生 `EventSource` 无法设置自定义 Header。为了保证长连接安全建立，服务设计了 **Ticket（用后即焚临时凭证）机制**：

### 1. 申请 Ticket (三方系统代办)
客户端（前端）向业务系统请求实时订阅时，业务系统用其 JWT Token 代为申请 Ticket：
* **POST** `/v1/hub/ticket/apply`
* **Payload (JSON)**:
```json
{
  "tenant": "tenantA",
  "user_id": "user123"
}
```
* **Response (JSON)**:
```json
{
  "code": 0,
  "msg": "凭证申请成功",
  "data": {
    "ticket": "692ea400c436b76174a74288bf7d02",  // 一次性凭证
    "expire_time": "2026-07-18T17:20:00+08:00",
    "create_time": "2026-07-18T17:19:30+08:00"
  }
}
```
*注：Ticket 的有效期通常为 30 秒（可在配置 `ticket.expire_seconds` 中调整），且建立连接后立刻在 Redis 中销毁。*

### 2. 建立 SSE 长连接
前端收到 `ticket` 后，直接发起原生 EventSource 请求：
```javascript
const sseUrl = `http://localhost:8080/v1/hub/subscribe?Ticket=692ea400c436b76174a74288bf7d02`;
const eventSource = new EventSource(sseUrl);

eventSource.onmessage = (event) => {
    const payload = JSON.parse(event.data);
    console.log("收到实时通知:", payload);
};

eventSource.onerror = (err) => {
    console.error("SSE 连接断开或异常", err);
};
```

---

## 🚀 gRPC 接口接入

针对内部微服务的高频投递场景，服务提供了高性能的 gRPC 通道。

### 1. Protobuf 定义
服务定义文件位于 `api/proto/notify.proto`，主要定义如下：

```protobuf
syntax = "proto3";

package proto;
option go_package = "./proto";

service NotifyService {
  rpc SendToUser (SendToUserRequest) returns (SendResponse);
  rpc SendToUsers (SendToUsersRequest) returns (SendResponse);
  rpc Broadcast (BroadcastRequest) returns (SendResponse);
}

message SendToUserRequest {
  string tenant_id = 1;
  string user_id = 2;
  string title = 3;
  string content = 4;
}

message SendToUsersRequest {
  string tenant_id = 1;
  repeated string user_ids = 2;
  string title = 3;
  string content = 4;
}

message BroadcastRequest {
  string tenant_id = 1;
  string title = 2;
  string content = 3;
}

message SendResponse {
  int32 code = 1;
  string msg = 2;
  string notify_id = 3; // 生成的消息ID
}
```

### 2. gRPC 客户端携带凭证 (Go 示例)
在调用 gRPC 方法时，必须在上下文（Metadata）中注入 `authorization: Bearer <Your_JWT_Token>`：

```go
package main

import (
	"context"
	"log"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"

	"bs-notify-hub/api/proto"
)

func main() {
	conn, err := grpc.Dial("localhost:9090", grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("连接 gRPC 失败: %v", err)
	}
	defer conn.Close()

	client := proto.NewNotifyServiceClient(conn)

	// 1. 准备你的 JWT Token (由授权中心签发)
	token := "Your_RS256_JWT_Token_Here"

	// 2. 注入 metadata (注意：HTTP 头是 Authorization，gRPC metadata 中统一为小写 authorization)
	ctx := metadata.NewOutgoingContext(
		context.Background(),
		metadata.Pairs("authorization", "Bearer "+token),
	)

	// 3. 发送调用
	resp, err := client.SendToUser(ctx, &proto.SendToUserRequest{
		TenantId: "tenantA",
		UserId:   "user123",
		Title:    "系统预警",
		Content:  "CPU 占用率过高 (95%)，请注意检查。",
	})
	if err != nil {
		log.Fatalf("调用 SendToUser 失败: %v", err)
	}

	log.Printf("响应结果: code=%d, msg=%s, id=%s", resp.Code, resp.Msg, resp.NotifyId)
}
```
