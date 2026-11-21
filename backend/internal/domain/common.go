// Package domain 定义全项目领域结构体：XxxDO(持久化)、XxxDTO(层间传输)、XxxVO(视图)、XxxREQ(请求)、XxxRESP(响应)。
// 字段命名全部使用 xxx_name / xxx_code / xxx_ulid 等显式后缀，避免 name、code 等歧义命名。
package domain

import "time"

// ULID 业务前缀常量：所有主键形如 PRJ_01HXXX，便于肉眼识别与日志排错。
const (
	IDPrefixUser            = "USR_" // 用户
	IDPrefixRequest         = "REQ_" // 请求链路 ID
	IDPrefixProject         = "PRJ_" // 小说项目
	IDPrefixVolume          = "VOL_" // 卷
	IDPrefixChapter         = "CHP_" // 章节
	IDPrefixChapterSnapshot = "SNP_" // 章节快照
	IDPrefixCharacter       = "CHR_" // 角色
	IDPrefixRelation        = "RLT_" // 角色关系
	IDPrefixLore            = "LOR_" // 世界观设定
	IDPrefixDocument        = "DOC_" // 知识库文档
	IDPrefixChunk           = "CHK_" // 知识库切片
	IDPrefixSession         = "SES_" // 会话
	IDPrefixMessage         = "MSG_" // 消息
	IDPrefixSkill           = "SKL_" // 技能
	IDPrefixSkillReference  = "SKR_" // 技能参考资料
	IDPrefixMenu            = "MNU_" // 菜单
	IDPrefixJob             = "JOB_" // 异步任务
	IDPrefixSetting         = "SET_" // 应用配置
	IDPrefixWordStat        = "WCS_" // 字数统计
	IDPrefixRole            = "ROL_" // 角色
	IDPrefixUserRole        = "URO_" // 用户-角色关联
	IDPrefixAudit           = "AUD_" // 审计日志
	IDPrefixModelProvider   = "MPR_" // 模型供应商
	IDPrefixModelCatalog    = "MDL_" // 模型目录
	IDPrefixModelGrant      = "MGR_" // 模型角色授权
	IDPrefixModelUsage      = "MUS_" // 模型用量统计
	IDPrefixNotification    = "NTF_" // 站内通知
	IDPrefixLoginLog        = "LGL_" // 登录日志
	IDPrefixPromptProfile   = "PMP_" // 系统提示词档案
)

// 业务响应码（字符串，与 HTTP 状态码 int 区分）：SUCCESS 与 ERR_ 前缀族。
const (
	RespCodeSuccess         = "SUCCESS"           // 成功
	RespCodeInvalidParam    = "ERR_INVALID_PARAM" // 参数校验失败
	RespCodeUnauthorized    = "ERR_UNAUTHORIZED"  // 未登录或登录失效
	RespCodeForbidden       = "ERR_FORBIDDEN"     // 无权限
	RespCodeNotFound        = "ERR_NOT_FOUND"     // 资源不存在
	RespCodeConflict        = "ERR_CONFLICT"      // 资源冲突（重名/重复提交）
	RespCodeBizFailed       = "ERR_BIZ_FAILED"    // 通用业务失败
	RespCodeAIProviderError = "ERR_AI_PROVIDER"   // AI 模型调用失败
	RespCodeStorageError    = "ERR_STORAGE"       // 对象存储/搜索/队列等基础设施错误
	RespCodeInternalError   = "ERR_INTERNAL"      // 服务内部错误
)

// BaseDO 持久化对象公共字段：时间戳由 GORM 自动维护，操作人由 service 层显式写入。
// 不使用软删除：业务删除是硬删除，且 user_name/role_code/menu_code 等列带唯一索引，
// 软删除会让已删行长期占位、无法重建同名资源。
type BaseDO struct {
	CreatedAt time.Time `json:"created_at" gorm:"autoCreateTime;column:created_at"`     // 创建时间
	UpdatedAt time.Time `json:"updated_at" gorm:"autoUpdateTime;column:updated_at"`     // 更新时间
	CreatedBy string    `json:"created_by" gorm:"column:created_by;size:40;default:''"` // 创建人 ULID（系统写入为空）
	UpdatedBy string    `json:"updated_by" gorm:"column:updated_by;size:40;default:''"` // 最后更新人 ULID
}

// BaseRESP 所有业务响应的公共包络。
type BaseRESP struct {
	RespCode    string `json:"resp_code"`    // 业务响应码
	RespMessage string `json:"resp_message"` // 人类可读提示
	TraceID     string `json:"trace_id"`     // 链路追踪 ID
}

// DataRESP 带业务数据体的响应。
type DataRESP[T any] struct {
	BaseRESP
	Data T `json:"data"` // 业务数据
}

// PageREQ 分页查询通用请求。
type PageREQ struct {
	PageNum  int `json:"page_num" form:"page_num"`   // 页码，从 1 开始
	PageSize int `json:"page_size" form:"page_size"` // 每页条数，最大 100
}

// Normalize 校验并补全默认分页参数。
func (p *PageREQ) Normalize() {
	if p.PageNum <= 0 {
		p.PageNum = 1 // 默认首页
	}
	if p.PageSize <= 0 || p.PageSize > 100 {
		p.PageSize = 20 // 默认每页 20，上限 100
	}
}

// PageDTO 分页结果（service → api）。
type PageDTO[T any] struct {
	Total    int64 `json:"total"`     // 总条数
	PageNum  int   `json:"page_num"`  // 当前页码
	PageSize int   `json:"page_size"` // 每页条数
	Items    []T   `json:"items"`     // 当前页数据
}

// NewPageDTO 构造分页结果。
func NewPageDTO[T any](total int64, req PageREQ, items []T) PageDTO[T] {
	return PageDTO[T]{Total: total, PageNum: req.PageNum, PageSize: req.PageSize, Items: items}
}
