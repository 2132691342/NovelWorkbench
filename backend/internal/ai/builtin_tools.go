// 本文件定义内置 AI 工具资产：工具是对话期可被显式调用（#code）或关键词自动触发的能力单元，
// 结果以「工具结果」上下文块注入模型，让模型依据事实作答而不是自己心算。
// 全部内置工具均为服务端确定性实现（不额外消耗模型调用），可在管理端停用或改描述。

package ai

import (
	"crypto/sha256"
	"encoding/hex"
	"strconv"
	"strings"
)

// BuiltinTool 内置工具定义。
type BuiltinTool struct {
	ToolCode        string // 工具编码（对话中以 #code 触发）
	ToolName        string // 工具名称
	DescriptionText string // 一句话说明（模型与用户可见）
	TriggerKeywords string // 自动触发关键词（逗号分隔，命中即运行）
	ParamList       []ToolParam
	SortOrder       int // 排序号
}

// ToolParam 工具参数声明。
type ToolParam struct {
	ParamName       string // 参数名
	ParamLabel      string // 展示名
	RequiredFlag    bool   // 是否必填
	DefaultValue    string // 缺省值
	DescriptionText string // 填写提示
}

// builtinToolList 内置工具清单（顺序即管理端展示顺序）。
var builtinToolList = []BuiltinTool{
	{
		ToolCode:        "chapter_check",
		ToolName:        "章节体检",
		DescriptionText: "按量化门禁检查正文：字数、破折号、零容忍词、情绪命名、对话标签与章末钩子，输出问题清单与结论",
		TriggerKeywords: "体检,检查这章,能不能发,过稿,审一下,查一下这章",
		SortOrder:       10,
		ParamList: []ToolParam{
			{ParamName: "content_text", ParamLabel: "正文", DescriptionText: "待检正文，留空则取当前章节"},
		},
	},
	{
		ToolCode:        "deai_scan",
		ToolName:        "去 AI 味扫描",
		DescriptionText: "扫描 AI 写作痕迹：套路词、三段式排比、定性总结、均匀段落与形容词堆叠，给出可替换的具体改法",
		TriggerKeywords: "去ai味,AI味,太像AI,机器味,像AI写的",
		SortOrder:       20,
		ParamList: []ToolParam{
			{ParamName: "content_text", ParamLabel: "正文", DescriptionText: "待扫正文，留空则取当前章节"},
		},
	},
	{
		ToolCode:        "chapter_digest",
		ToolName:        "章节摘要",
		DescriptionText: "抽取式生成章节摘要：每段首句 + 章末钩子 + 出场角色，供连续性核对与 @摘要 引用",
		TriggerKeywords: "摘要,总结这章,概括这章",
		SortOrder:       30,
		ParamList: []ToolParam{
			{ParamName: "content_text", ParamLabel: "正文", DescriptionText: "待摘要正文，留空则取当前章节"},
			{ParamName: "max_sentences", ParamLabel: "句数上限", DefaultValue: "8", DescriptionText: "摘要最多保留几句"},
		},
	},
	{
		ToolCode:        "character_scan",
		ToolName:        "角色出场统计",
		DescriptionText: "统计本项目角色在正文中的出场次数与首末位置，识别本章缺席的主角与突然出现的配角",
		TriggerKeywords: "出场,谁出场,角色统计,人物统计",
		SortOrder:       40,
		ParamList: []ToolParam{
			{ParamName: "content_text", ParamLabel: "正文", DescriptionText: "待统计正文，留空则取当前章节"},
		},
	},
	{
		ToolCode:        "foreshadow_scan",
		ToolName:        "伏笔回收检查",
		DescriptionText: "列出项目内未回收的伏笔钩子，并检查本章是否出现相关关键词，提示哪些该收、哪些放久了",
		TriggerKeywords: "伏笔,回收,挖坑,坑填了没",
		SortOrder:       50,
		ParamList: []ToolParam{
			{ParamName: "content_text", ParamLabel: "正文", DescriptionText: "待检正文，留空则取当前章节"},
		},
	},
	{
		ToolCode:        "lore_lookup",
		ToolName:        "设定检索",
		DescriptionText: "按关键词检索本项目世界观设定与知识库文档片段，返回原文与出处，避免模型编造设定",
		TriggerKeywords: "设定是什么,查设定,世界观,资料里怎么写的",
		SortOrder:       60,
		ParamList: []ToolParam{
			{ParamName: "query_text", ParamLabel: "检索词", RequiredFlag: true, DescriptionText: "要查的设定或名词"},
		},
	},
}

// BuiltinTools 返回全部内置工具定义（只读副本）。
func BuiltinTools() []BuiltinTool {
	tools := make([]BuiltinTool, len(builtinToolList))
	copy(tools, builtinToolList)
	for i := range tools {
		tools[i].ParamList = append([]ToolParam(nil), builtinToolList[i].ParamList...)
	}
	return tools
}

// BuiltinToolCodeSet 返回内置工具编码集合（播种时清理已下线工具）。
func BuiltinToolCodeSet() map[string]bool {
	codeSet := make(map[string]bool, len(builtinToolList))
	for _, tool := range builtinToolList {
		codeSet[tool.ToolCode] = true
	}
	return codeSet
}

// BuiltinToolOf 按编码返回内置工具定义；不存在时返回 nil。
func BuiltinToolOf(toolCode string) *BuiltinTool {
	for i := range builtinToolList {
		if builtinToolList[i].ToolCode == toolCode {
			tool := builtinToolList[i]
			tool.ParamList = append([]ToolParam(nil), builtinToolList[i].ParamList...)
			return &tool
		}
	}
	return nil
}

// ContentHash 计算内置工具出厂指纹（编码/名称/说明/关键词/参数/排序）。
func (t BuiltinTool) ContentHash() string {
	paramText := make([]string, 0, len(t.ParamList)*4)
	for _, param := range t.ParamList {
		paramText = append(paramText, param.ParamName, param.ParamLabel,
			strconv.FormatBool(param.RequiredFlag), param.DefaultValue, param.DescriptionText)
	}
	digest := sha256.Sum256([]byte(strings.Join([]string{
		"tool", t.ToolCode, t.ToolName, t.DescriptionText, t.TriggerKeywords,
		strconv.Itoa(t.SortOrder), strings.Join(paramText, "\x1f"),
	}, "\x1e")))
	return hex.EncodeToString(digest[:])
}

// BuiltinToolHashOf 返回指定内置工具的出厂指纹；不在清单中返回空串。
func BuiltinToolHashOf(toolCode string) string {
	if tool := BuiltinToolOf(toolCode); tool != nil {
		return tool.ContentHash()
	}
	return ""
}

// ToolKeywordsOf 返回工具的触发关键词列表（逗号分隔，已去空白）。
func ToolKeywordsOf(triggerKeywords string) []string {
	parts := strings.Split(triggerKeywords, ",")
	keywords := make([]string, 0, len(parts))
	for _, part := range parts {
		if trimmed := strings.TrimSpace(part); trimmed != "" {
			keywords = append(keywords, trimmed)
		}
	}
	return keywords
}
