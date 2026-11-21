// 本文件定义站内通知的持久化对象、查询/已读/删除请求与视图对象。

package domain

// 通知类型编码常量。
const (
	NotificationTypeSystem  = "system"  // 系统公告
	NotificationTypeTask    = "task"    // 异步任务结果（知识库索引等）
	NotificationTypeAccount = "account" // 账号事件（创建、密码变更等）
)

// 通知已读状态过滤值（仅用于查询）。
const (
	NotificationReadStatusAll    = "all"    // 全部
	NotificationReadStatusUnread = "unread" // 未读
	NotificationReadStatusRead   = "read"   // 已读
)

// NotificationDO 站内通知持久化对象（按用户隔离，读后保留）。
type NotificationDO struct {
	BaseDO
	NotificationULID      string `json:"notification_ulid" gorm:"column:notification_ulid;primaryKey;size:40"`      // 通知 ULID（NTF_ 前缀）
	UserULID              string `json:"user_ulid" gorm:"column:user_ulid;size:40;index"`                           // 接收用户
	NotificationTypeCode  string `json:"notification_type_code" gorm:"column:notification_type_code;size:16;index"` // 类型：system/task/account
	NotificationTitleText string `json:"notification_title_text" gorm:"column:notification_title_text;size:128"`    // 标题
	NotificationBodyText  string `json:"notification_body_text" gorm:"column:notification_body_text;size:512"`      // 正文
	ReadFlag              bool   `json:"read_flag" gorm:"column:read_flag;index;default:false"`                     // 是否已读
	LinkURL               string `json:"link_url" gorm:"column:link_url;size:256"`                                  // 点击跳转地址（前端路由）
	RefTypeCode           string `json:"ref_type_code" gorm:"column:ref_type_code;size:32"`                         // 关联业务类型
	RefULID               string `json:"ref_ulid" gorm:"column:ref_ulid;size:40"`                                   // 关联业务 ULID
}

// TableName 指定表名。
func (NotificationDO) TableName() string { return "notifications" }

// NotificationCreateDTO 内部创建通知的载荷（业务服务之间传递）。
type NotificationCreateDTO struct {
	UserULID              string // 接收用户
	NotificationTypeCode  string // 类型
	NotificationTitleText string // 标题
	NotificationBodyText  string // 正文
	LinkURL               string // 跳转地址
	RefTypeCode           string // 关联业务类型
	RefULID               string // 关联业务 ULID
}

// NotificationListREQ 通知分页查询请求。
type NotificationListREQ struct {
	PageREQ
	ReadStatusCode       string `json:"read_status_code" binding:"omitempty,oneof=all unread read"` // 已读状态：all/unread/read
	NotificationTypeCode string `json:"notification_type_code" binding:"omitempty,max=16"`          // 类型过滤
	KeywordText          string `json:"keyword_text" binding:"omitempty,max=64"`                    // 标题/正文模糊
}

// NotificationReadREQ 批量标记已读请求。
type NotificationReadREQ struct {
	NotificationULIDList []string `json:"notification_ulid_list" binding:"required,min=1,max=100"` // 通知 ULID 列表
}

// NotificationDeleteREQ 删除单条通知请求。
type NotificationDeleteREQ struct {
	NotificationULID string `json:"notification_ulid" binding:"required,max=40"` // 通知 ULID
}

// NotificationVO 通知视图对象。
type NotificationVO struct {
	NotificationULID      string `json:"notification_ulid"`       // 通知 ULID
	NotificationTypeCode  string `json:"notification_type_code"`  // 类型
	NotificationTitleText string `json:"notification_title_text"` // 标题
	NotificationBodyText  string `json:"notification_body_text"`  // 正文
	ReadFlag              bool   `json:"read_flag"`               // 是否已读
	LinkURL               string `json:"link_url"`                // 跳转地址
	RefTypeCode           string `json:"ref_type_code"`           // 关联业务类型
	RefULID               string `json:"ref_ulid"`                // 关联业务 ULID
	CreatedAt             string `json:"created_at"`              // 创建时间（RFC3339）
}

// NotificationUnreadCountVO 未读数量视图。
type NotificationUnreadCountVO struct {
	UnreadCount int64 `json:"unread_count"` // 未读条数
}
