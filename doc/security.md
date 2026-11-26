# 安全设计

本文说明认证、授权、密钥、审计、登录防护、上传与部署边界。
安全目标：**账号由管理员分配、权限逐请求校验、密钥不出明文、关键动作可追溯。**

## 1. 认证

| 项 | 设计 |
| --- | --- |
| 登录方式 | 登录名或邮箱 + 密码（`UserRepo.GetByAccount`） |
| 密码存储 | bcrypt DefaultCost，API 永不返回 `password_hash` |
| 会话 | JWT HS256，默认 7 天，载荷含 `user_ulid`、`pwd_epoch`、签发与过期时间 |
| 登出 | 令牌写入 Redis 黑名单，TTL 取 `expires_at - issued_at`（签发时长） |
| 账号禁用 | 每次请求校验用户状态，禁用后旧令牌立即失效 |
| 改密失效 | `users.pwd_epoch` 递增，令牌内版本落后立即 401；本人改密返回新令牌续期 |
| 注册 | 不开放，账号只能由管理员创建 |
| 个人改密 | 必须校验旧密码，成功后写安全通知 |

登录结果处理（`AuthService.Login`）：

| 情况 | 响应 | 登录日志 |
| --- | --- | --- |
| 账号不存在 | `ERR_UNAUTHORIZED`「账号或密码错误」 | 记录，原因「账号不存在」 |
| 密码错误 | 同上，文案一致，不泄露账号是否存在 | 记录，原因「密码错误」 |
| 账号禁用 | `ERR_FORBIDDEN`「账号已被禁用」 | 记录，结果 disabled |
| 成功 | 返回令牌、有效期与用户信息 | 记录，结果 success |

登录日志写入失败只告警，不阻断登录。

令牌校验链路（`pkg.AuthMiddleware`）：

```
Authorization: Bearer <token>
  → 解析签名与过期时间（pkg.JWTSigner.ParseClaims）
  → 查 Redis 黑名单（pkg.TokenStore.IsRevoked）
  → 校验用户状态与密码版本（不存在或禁用 → 401）
  → 令牌内 pwd_epoch 落后于库内值 → 401「密码已变更，请重新登录」
  → 注入 user_ulid
```

### 1.1 写请求幂等

重复提交是写作类系统最容易被忽略的闭环：网络重试、双击、弱网重发都会造成重复写入。

| 层 | 手段 | 说明 |
| --- | --- | --- |
| 前端 | 每次 POST 生成 `Idempotency-Key` | 26 位 ULID，`utils/idempotency.ts` |
| 前端 | 写动作单飞合并 | 相同 URL + 相同请求体在途时复用同一 Promise |
| 后端 | `pkg.IdempotencyMiddleware` | Redis `SET NX` 抢占，成功响应缓存 24 小时 |
| 后端 | 同键不同体拒绝 | `ERR_INVALID_PARAM`，避免把两次动作混成一次 |
| 后端 | 只缓存 2xx | 失败释放键，允许改正后重试 |
| 后端 | 删除幂等 | 目标已不存在即视为成功，越权仍 403 |

查询类 POST（`/list`、`/history`、`/detail`）**不进入**幂等白名单：缓存列表响应会让用户在 24 小时内看不到自己刚写的数据。

## 2. 授权

### 2.1 权限码单一事实源

- 权限码登记在 `menus.perm_code`，页面行与动作行都可承载权限。
- 角色 `roles.perm_codes_json` 保存勾选的权限码，`*` 表示全部权限。
- 导航树与权限目录都由菜单表推导，新增权限只需登记一行菜单。
- 接口侧由 `pkg.RequirePermMiddleware` 逐请求查库（`RoleService.HasPerm`），角色变更立即生效；前端路由 `meta.perm_code` 只是体验层，不作为安全边界。

### 2.2 权限码

| 权限码 | 语义 | 收口范围 |
| --- | --- | --- |
| `*` | 全部权限 | 内置 admin 角色 |
| `system:user:manage` | 用户管理 | `/admin/user/*` |
| `system:role:manage` | 角色管理 | `/admin/role/*` |
| `system:menu:manage` | 菜单管理 | `/menu/list|detail|save|delete` |
| `system:perm:read` | 权限目录 | `/menu/permission/catalog` |
| `system:audit:read` | 审计日志 | `/admin/audit/list` |
| `system:loginlog:read` | 登录日志 | `/admin/login_log/list` |
| `system:job:manage` | 任务中心 | `/admin/job/*` |
| `system:model:manage` | 模型生态 | `/model/provider/*`、`/model/catalog/*` |
| `system:usage:read` | 用量分析 | `/model/usage/overview` |
| `system:status:read` | 运行状态 | `/system/status` |
| `system:setting:manage` | 运行时配置 | `/setting/update` |
| `prompt:manage` | 提示词库 | `/prompt/save`、`/prompt/delete` |
| `tool:manage` | 工具资产 | `/tool/save`、`/tool/delete` |

权限码常量定义在 `backend/internal/domain/role.go`。

### 2.3 数据归属

- 项目是创作资源隔离边界，所有创作表带 `project_ulid`。
- service 层统一校验归属：不存在返回 404，非本人返回 403。
- 通知、登录日志、个人资料按 `user_ulid` 隔离。
- 知识库全局文档（`project_ulid = "0"`）可被所有项目引用，但仍需登录。
- 任务中心、菜单、用户、角色等管理接口按权限码收口。

## 3. 密钥与配置

| 密钥 | 存储 | 说明 |
| --- | --- | --- |
| JWT Secret | `NW_JWT_SECRET` | 同时作为模型密钥封存材料 |
| 模型 API Key | AES-256-GCM 加密入库（`pkg.SealSecret`） | 仅回显掩码，解密只发生在调用瞬间 |
| 中间件凭据 | `.env` 注入 | 仓库内配置文件全部留空 |
| 管理员初始密码 | `NW_APP_ADMIN_PASSWORD` | 仅用户表为空时生效 |

规则：

- `backend/configs/config.yaml` 只保留空值模板，真实凭据一律走环境变量。
- `.env` 已被 `.gitignore` 忽略，示例文件只放占位值。
- 日志禁止输出密码、令牌与 API Key 原文；部署脚本过滤含 `password` 的行。
- 更换 `NW_JWT_SECRET` 会使既有令牌与全部模型密钥失效，需要重新登录并重填供应商密钥。

密钥封存实现（`backend/internal/pkg/secret.go`）：

| 项 | 说明 |
| --- | --- |
| 算法 | AES-256-GCM，密钥由 SHA-256(材料) 派生 |
| 输出 | `base64(nonce ‖ 密文)`，nonce 每次随机 |
| 失败 | 空材料返回 `ErrSecretEmpty`；密文非法或材料不匹配返回 `ErrSecretInvalid` |

## 4. 审计与留痕

### 4.1 审计日志

- 所有 POST 请求（除登录、登出、`/chat/send`）记录一条审计，无论成败。
- **存储位置是 Manticore 全文表**（`nw_audit`），不在 PostgreSQL。全文表天然支持 CJK ngram 检索与毫秒时间范围过滤，列表页的关键字、动作模糊与链路查询都由它直接完成；PostgreSQL 里的历史审计表在启动时自动清理。
- 每日 03:00 由后台任务清理超过留存天数的记录（默认 30 天，`audit_retention_days` 可调），审计页也提供「立即清理」。
- 字段：级别（`info` / `warn` / `error`，按 HTTP 状态推导）、链路 ID（与响应包络 `trace_id` 同值）、操作者 ULID 与登录名快照、动作码（路径去掉 `/api/v1/`）、请求路径与方法、脱敏请求体、**脱敏响应体**、目标类型与目标 ULID、业务响应码与说明、HTTP 状态、耗时、IP、UA。
- 请求体与响应体都采样落库（各 32KB 上限，超限追加截断标记）；响应体非 JSON（SSE 流、上传回执）记占位说明。详情接口 `AuditDetailVO` 同时返回 `request_body` 与 `resp_body`，前端抽屉并列展示——「只记请求不记响应」无法回答「这次写到底改成了什么」。
- 所有 POST 都读取 body 提取目标，上限 32KB，读后还原供 handler 绑定；`multipart` 请求不采样。
- 目标提取白名单覆盖 user / role / menu / project / chapter / document / prompt / tool / model / provider / chat_session / notification，取第一个命中的键。
- 凭据字段（`password_text`、`new_password_text`、`old_password_text`、`api_key_text`、`access_token`）一律替换为 `***`，审计表即使被读也不泄露口令。
- 写入失败只告警（`audit_record_failed`），不阻断业务；Manticore 不可用时审计静默丢弃并告警，业务照常。
- **链路时间线**：同一 `trace_id` 的全部审计行可用 `/admin/audit/trace/{TraceID}` 一次取回，前端按时间升序画成时间线；工作流运行复用创建它的那次请求的链路 ID，因此一次「启动工作流」能顺着链路查到后续所有步骤。

### 4.2 登录日志

成功、密码错误、账号不存在、账号禁用四种结果全部留痕，用于排查撞库、异常 IP 与离职账号尝试。管理端需 `system:loginlog:read`，支持账号 / IP / 结果 / 日期筛选。

## 5. 上传与内容安全

| 风险 | 控制 |
| --- | --- |
| 超大文件 | `NW_APP_UPLOAD_MAX_MB` 限制，服务端再次校验 |
| 非法类型 | 仅 txt / md / docx / pdf，扩展名与内容解析双重校验 |
| 路径穿越 | 对象键由服务端生成（= `doc_ulid`），不使用用户文件名 |
| 文档解析 | 纯 Go 解析 docx / pdf，不落地可执行临时文件 |
| XSS | 后端只存文本；富文本渲染由前端组件负责 |
| 对象存储 | 原文私有桶，前端经后端字节流预览 |

## 6. 传输与部署边界

- 对外只暴露前端入口端口（默认 14399）；后端 8080 与各中间件端口只在容器网络或本机监听。
- 生产建议在入口前做 TLS 终止（云负载均衡或 nginx），禁止明文公网传输密码。
- CORS 当前允许所有来源以支持前后端分离开发；固定域名部署时应收敛 `AllowOrigins`。
- 方法白名单只允许 GET / POST；删除通过 `POST /xxx/delete`。
- 容器以非 root 用户运行，中间件凭据由编排注入。

## 7. 威胁与对策

| 威胁 | 影响 | 对策 |
| --- | --- | --- |
| 撞库 | 账号被猜解 | bcrypt、统一失败文案、登录日志、管理员禁用 |
| 令牌泄露 | 会话被冒用 | 可配有效期、登出黑名单、禁用即时失效 |
| 越权访问项目 | 读到他人作品 | 每个创作接口校验 `project_ulid` 归属 |
| 越权管理 | 修改用户 / 菜单 | 权限码逐请求查库，前端守卫仅作体验层 |
| 密钥泄露 | 模型额度被盗 | AES-GCM 加密、掩码回显、日志脱敏 |
| 恶意文件 | 解析漏洞 / 资源耗尽 | 类型白名单、大小限制、纯 Go 解析、异步索引 |
| 审计缺失 | 无法追责 | 写操作全量审计，登录全量留痕 |
| 毒消息阻塞队列 | worker 卡死 | 内部重试上限 3 次、失败落库并 ack、任务中心可重试 |

## 8. 运维清单

- 生产替换 `NW_JWT_SECRET`、`NW_APP_ADMIN_PASSWORD` 与全部中间件口令。
- 确认 14399 之外的后端与中间件端口未对公网开放。
- 配置 TLS 终止与安全响应头（HSTS、`X-Content-Type-Options`）。
- 定期检查登录日志中的异常 IP 与失败峰值。
- 定期抽查审计日志中的高危动作（删除项目、改角色、重置密码）。
- 备份 PostgreSQL、MinIO 桶与 Manticore 索引配置。
- 离职账号及时禁用，禁用后旧令牌自动失效。
- 模型供应商密钥按周期轮换，轮换后需在页面重填。
