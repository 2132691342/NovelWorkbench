// 本文件维护动作码 → 中文动作名的映射：审计列表与链路时间线靠它把动作码翻译成人话。
// 未登记的编码回落到编码本身，避免出现空标签；后台步骤与 AI 调用按前缀归类。

package pkg

import "strings"

// auditActionNameMap 已登记的 HTTP 动作码与系统动作码（键为路径去掉 /api/v1/ 前缀后的编码）。
var auditActionNameMap = map[string]string{
	// 认证与个人资料
	"auth/login":              "登录",
	"auth/logout":             "登出",
	"profile/update":          "修改个人资料",
	"profile/password/change": "修改密码",
	// 通知
	"notification/list":     "查看通知",
	"notification/read":     "标记通知已读",
	"notification/read_all": "全部标记已读",
	"notification/delete":   "删除通知",
	// 项目
	"project/create":          "创建项目",
	"project/update":          "修改项目",
	"project/delete":          "删除项目",
	"project/list":            "查看项目列表",
	"project/bootstrap/plan":  "AI 开书方案",
	"project/bootstrap/apply": "应用开书方案",
	// 卷章
	"volume/create":            "创建卷",
	"volume/update":            "修改卷",
	"volume/delete":            "删除卷",
	"chapter/create":           "创建章节",
	"chapter/update":           "修改章节",
	"chapter/delete":           "删除章节",
	"chapter/check":            "章节体检",
	"chapter/snapshot/create":  "创建章节快照",
	"chapter/snapshot/restore": "恢复章节快照",
	// 角色与设定
	"character/save":            "保存角色",
	"character/delete":          "删除角色",
	"character/list":            "查看角色列表",
	"character/relation/save":   "保存角色关系",
	"character/relation/delete": "删除角色关系",
	"lore/save":                 "保存设定",
	"lore/delete":               "删除设定",
	"lore/list":                 "查看设定列表",
	// 知识库
	"knowledge/upload":  "上传知识库文档",
	"knowledge/list":    "查看知识库",
	"knowledge/delete":  "删除知识库文档",
	"knowledge/search":  "检索知识库",
	"knowledge/reindex": "重建知识库索引",
	// 提示词与工具
	"prompt/save":    "保存提示词",
	"prompt/delete":  "删除提示词",
	"prompt/resolve": "解析提示词",
	"tool/invoke":    "调用工具",
	"tool/save":      "保存工具",
	"tool/delete":    "删除工具",
	// 菜单与配置
	"menu/save":      "保存菜单",
	"menu/delete":    "删除菜单",
	"setting/update": "修改系统配置",
	// 模型管理
	"model/provider/save":       "保存模型供应商",
	"model/provider/delete":     "删除模型供应商",
	"model/provider/test":       "测试模型供应商",
	"model/catalog/save":        "保存模型",
	"model/catalog/delete":      "删除模型",
	"model/catalog/test":        "测试模型",
	"model/catalog/set_default": "设为默认模型",
	"model/catalog/grant":       "调整模型授权",
	// 大纲与连续性
	"outline/node/save":       "保存大纲节点",
	"outline/node/delete":     "删除大纲节点",
	"outline/scene/save":      "保存场景卡",
	"outline/scene/delete":    "删除场景卡",
	"continuity/fact/save":    "保存时间事实",
	"continuity/fact/review":  "审核时间事实",
	"continuity/hook/save":    "保存伏笔",
	"continuity/summary/save": "保存章节摘要",
	// 工作流
	"workflow/run/create":      "创建工作流",
	"workflow/run/quick_start": "一键开跑",
	"workflow/run/list":        "查看工作流列表",
	"workflow/run/approve":     "通过工作流审核",
	"workflow/run/reject":      "退回工作流",
	"workflow/run/pause":       "暂停工作流",
	"workflow/run/resume":      "继续工作流",
	"workflow/run/cancel":      "取消工作流",
	"workflow/run/retry":       "重试工作流",
	// 管理端
	"admin/user/list":           "查看用户列表",
	"admin/user/create":         "创建用户",
	"admin/user/update":         "修改用户",
	"admin/user/delete":         "删除用户",
	"admin/user/reset_password": "重置用户密码",
	"admin/role/list":           "查看角色列表",
	"admin/role/save":           "保存角色",
	"admin/role/delete":         "删除角色",
	"admin/audit/list":          "查看审计日志",
	"admin/audit/trace/list":    "查看审计链路",
	"admin/audit/stats":         "查看审计概览",
	"admin/audit/purge":         "清理审计日志",
	"admin/login_log/list":      "查看登录日志",
	"admin/job/list":            "查看任务列表",
	"admin/job/retry":           "重试异步任务",
	// AI 调用与系统动作
	"ai/chat":            "AI 对话",
	"ai/bootstrap/plan":  "AI 开书方案",
	"system/audit.purge": "清理审计日志",
	// 对话与项目查询
	"chat/send":                 "AI 对话",
	"chat/history":              "查看对话记录",
	"chat/session/list":         "查看会话列表",
	"chat/session/create":       "新建会话",
	"chat/session/delete":       "删除会话",
	"project/detail":            "查看项目详情",
	"project/tree":              "查看目录树",
	"chapter/detail":            "查看章节正文",
	"chapter/snapshot/list":     "查看章节快照",
	"setting/list":              "查看系统配置",
	"menu/nav":                  "获取导航菜单",
	"tool/list":                 "查看工具列表",
	"prompt/list":               "查看提示词列表",
	"prompt/detail":             "查看提示词详情",
	"model/capability":          "查看可用模型",
	"model/usage/overview":      "查看用量分析",
	"notification/unread_count": "查看未读通知数",
	"profile/detail":            "查看个人资料",
	"stats/overview":            "查看数据总览",
	"system/status":             "查看运行状态",
}

// auditStepNameMap 工作流步骤类型 → 中文动作名（步骤审计与 AI 步骤审计共用）。
var auditStepNameMap = map[string]string{
	"draft":        "AI 出稿",
	"revise":       "AI 修订",
	"review":       "AI 复审",
	"summary":      "AI 摘要",
	"check":        "量化体检",
	"apply":        "定稿入库",
	"human_review": "人工审核",
}

// AuditActionNameOf 返回动作码对应的中文动作名；未登记时返回空串，由调用方回落到动作码。
func AuditActionNameOf(actionCode string) string {
	code := strings.TrimSpace(actionCode)
	if code == "" {
		return ""
	}
	if name, ok := auditActionNameMap[code]; ok {
		return name
	}
	// 工作流步骤：workflow/step/<类型>（步骤执行）与 ai/workflow/<类型>（步骤内的模型调用）
	for _, prefix := range []string{"workflow/step/", "ai/workflow/"} {
		if stepType, ok := strings.CutPrefix(code, prefix); ok {
			if name, found := auditStepNameMap[stepType]; found {
				if prefix == "ai/workflow/" {
					return "AI " + strings.TrimPrefix(name, "AI ")
				}
				return name
			}
		}
	}
	return ""
}

// AuditActionLabel 返回用于展示的动作名：未登记时回落到动作码本身。
func AuditActionLabel(actionCode string) string {
	if name := AuditActionNameOf(actionCode); name != "" {
		return name
	}
	return actionCode
}
