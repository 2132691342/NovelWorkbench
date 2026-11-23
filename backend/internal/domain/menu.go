// 本文件定义菜单、权限目录与导航树的持久化对象、请求与视图对象。

package domain

// 菜单类型编码常量。
const (
	MenuTypeGroup  = "group"  // 分组标题（不可点击，仅组织下级）
	MenuTypePage   = "page"   // 页面（可点击跳转）
	MenuTypeAction = "action" // 动作（不显示，仅承载权限码）
)

// 菜单作用域编码常量。
const (
	MenuScopeGlobal  = "global"  // 全局菜单
	MenuScopeProject = "project" // 项目内菜单（路由携带 project_ulid）
)

// MenuDO 菜单持久化对象：同时是权限码的登记处（页面与动作各占一行）。
type MenuDO struct {
	BaseDO
	MenuULID      string `json:"menu_ulid" gorm:"column:menu_ulid;primaryKey;size:40"`  // 菜单 ULID（MNU_ 前缀）
	ParentULID    string `json:"parent_ulid" gorm:"column:parent_ulid;size:40;index"`   // 上级菜单 ULID，空为根
	MenuCode      string `json:"menu_code" gorm:"column:menu_code;size:64;uniqueIndex"` // 菜单编码（唯一）
	MenuName      string `json:"menu_name" gorm:"column:menu_name;size:64"`             // 菜单名称
	MenuTypeCode  string `json:"menu_type_code" gorm:"column:menu_type_code;size:16"`   // 类型：group/page/action
	ScopeCode     string `json:"scope_code" gorm:"column:scope_code;size:16;index"`     // 作用域：global/project
	RoutePath     string `json:"route_path" gorm:"column:route_path;size:128"`          // 前端路由路径（项目内以 :project_ulid 结尾）
	ComponentPath string `json:"component_path" gorm:"column:component_path;size:128"`  // 前端组件路径（如 views/Workspace/index，空为无组件）
	IconCode      string `json:"icon_code" gorm:"column:icon_code;size:32"`             // 图标编码（前端图标表键）
	PermCode      string `json:"perm_code" gorm:"column:perm_code;size:64;index"`       // 权限码（空表示登录即可见）
	VisibleFlag   bool   `json:"visible_flag" gorm:"column:visible_flag;default:true"`  // 是否在导航中展示
	EnabledFlag   bool   `json:"enabled_flag" gorm:"column:enabled_flag;default:true"`  // 是否启用
	SortOrder     int    `json:"sort_order" gorm:"column:sort_order"`                   // 同级排序号，升序
	RemarkText    string `json:"remark_text" gorm:"column:remark_text;size:256"`        // 备注
}

// TableName 指定表名。
func (MenuDO) TableName() string { return "menus" }

// MenuREQ 创建/更新菜单请求（menu_ulid 为空表示创建）。
type MenuREQ struct {
	MenuULID      string `json:"menu_ulid" binding:"omitempty,max=40"`                      // 菜单 ULID
	ParentULID    string `json:"parent_ulid" binding:"omitempty,max=40"`                    // 上级菜单 ULID
	MenuCode      string `json:"menu_code" binding:"required,min=2,max=64"`                 // 菜单编码
	MenuName      string `json:"menu_name" binding:"required,max=64"`                       // 菜单名称
	MenuTypeCode  string `json:"menu_type_code" binding:"required,oneof=group page action"` // 类型
	ScopeCode     string `json:"scope_code" binding:"omitempty,oneof=global project"`       // 作用域
	RoutePath     string `json:"route_path" binding:"omitempty,max=128"`                    // 路由路径
	ComponentPath string `json:"component_path" binding:"omitempty,max=128"`                // 组件路径（views/Xxx/index，空为无组件）
	IconCode      string `json:"icon_code" binding:"omitempty,max=32"`                      // 图标编码
	PermCode      string `json:"perm_code" binding:"omitempty,max=64"`                      // 权限码
	VisibleFlag   *bool  `json:"visible_flag"`                                              // 是否展示（nil 保持 true）
	EnabledFlag   *bool  `json:"enabled_flag"`                                              // 是否启用（nil 保持 true）
	SortOrder     int    `json:"sort_order" binding:"omitempty,min=0,max=9999"`             // 排序号
	RemarkText    string `json:"remark_text" binding:"omitempty,max=256"`                   // 备注
}

// MenuDeleteREQ 删除菜单请求。
type MenuDeleteREQ struct {
	MenuULID string `json:"menu_ulid" binding:"required,max=40"` // 菜单 ULID
}

// MenuListREQ 菜单列表查询请求（管理端，含禁用与隐藏项）。
type MenuListREQ struct {
	KeywordText  string `json:"keyword_text" form:"keyword_text" binding:"omitempty,max=64"`     // 名称/编码/权限码模糊
	MenuTypeCode string `json:"menu_type_code" form:"menu_type_code" binding:"omitempty,max=16"` // 类型过滤
	ScopeCode    string `json:"scope_code" form:"scope_code" binding:"omitempty,max=16"`         // 作用域过滤
}

// MenuVO 菜单视图对象（管理端平铺列表）。
type MenuVO struct {
	MenuULID      string `json:"menu_ulid"`      // 菜单 ULID
	ParentULID    string `json:"parent_ulid"`    // 上级菜单 ULID
	ParentName    string `json:"parent_name"`    // 上级菜单名称
	MenuCode      string `json:"menu_code"`      // 菜单编码
	MenuName      string `json:"menu_name"`      // 菜单名称
	MenuTypeCode  string `json:"menu_type_code"` // 类型
	ScopeCode     string `json:"scope_code"`     // 作用域
	RoutePath     string `json:"route_path"`     // 路由路径
	ComponentPath string `json:"component_path"` // 组件路径
	IconCode      string `json:"icon_code"`      // 图标编码
	PermCode      string `json:"perm_code"`      // 权限码
	VisibleFlag   bool   `json:"visible_flag"`   // 是否展示
	EnabledFlag   bool   `json:"enabled_flag"`   // 是否启用
	SortOrder     int    `json:"sort_order"`     // 排序号
	RemarkText    string `json:"remark_text"`    // 备注
	CreatedAt     string `json:"created_at"`     // 创建时间
	UpdatedAt     string `json:"updated_at"`     // 更新时间
}

// MenuTreeNodeVO 导航树节点（前端渲染侧边栏消费）。
type MenuTreeNodeVO struct {
	MenuCode      string           `json:"menu_code"`      // 菜单编码
	MenuName      string           `json:"menu_name"`      // 菜单名称
	MenuTypeCode  string           `json:"menu_type_code"` // 类型：group/page
	ScopeCode     string           `json:"scope_code"`     // 作用域
	RoutePath     string           `json:"route_path"`     // 路由路径
	ComponentPath string           `json:"component_path"` // 组件路径
	IconCode      string           `json:"icon_code"`      // 图标编码
	PermCode      string           `json:"perm_code"`      // 权限码（空为登录可见）
	VisibleFlag   bool             `json:"visible_flag"`   // 是否进导航（false 仅注册路由，深链仍可访问）
	ChildList     []MenuTreeNodeVO `json:"child_list"`     // 子节点
}

// MenuNavDTO 当前用户的导航数据（菜单树 + 权限码集合）。
type MenuNavDTO struct {
	MenuTreeList []MenuTreeNodeVO `json:"menu_tree_list"` // 可见菜单树
	PermCodeList []string         `json:"perm_code_list"` // 命中的权限码集合（"*" 表示全部）
}

// PermItemVO 权限目录条目。
type PermItemVO struct {
	PermCode    string `json:"perm_code"`    // 权限码
	PermName    string `json:"perm_name"`    // 权限名称
	PermDesc    string `json:"perm_desc"`    // 说明
	MenuCode    string `json:"menu_code"`    // 归属菜单编码（空为动作权限）
	MenuName    string `json:"menu_name"`    // 归属菜单名称
	BuiltinFlag bool   `json:"builtin_flag"` // 是否内置权限（不可删除）
}

// PermGroupVO 权限目录分组。
type PermGroupVO struct {
	GroupCode string       `json:"group_code"` // 分组编码
	GroupName string       `json:"group_name"` // 分组名称
	PermList  []PermItemVO `json:"perm_list"`  // 组内权限
}
