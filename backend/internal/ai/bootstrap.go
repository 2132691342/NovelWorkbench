// 本文件实现「AI 开书」：一句创意 → 结构化开书方案（书名、卖点、卷章、角色、设定）。
// 方案先给作者预览与修改，确认后才落库，避免模型幻觉直接污染作品数据。

package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/cloudwego/eino/schema"
)

// BootstrapPlan 开书方案（模型返回的原始结构，落库前由 service 归一化）。
type BootstrapPlan struct {
	ProjectName      string               `json:"project_name"`       // 书名
	GenreCode        string               `json:"genre_code"`         // 题材编码
	SellingPointText string               `json:"selling_point_text"` // 卖点
	SynopsisText     string               `json:"synopsis_text"`      // 简介
	StyleText        string               `json:"style_text"`         // 文风
	VolumeList       []BootstrapVolume    `json:"volume_list"`        // 卷与章
	CharacterList    []BootstrapCharacter `json:"character_list"`     // 角色卡
	LoreList         []BootstrapLore      `json:"lore_list"`          // 世界观设定
}

// BootstrapVolume 方案中的卷（含章节）。
type BootstrapVolume struct {
	VolumeName  string             `json:"volume_name"`
	SummaryText string             `json:"summary_text"`
	ChapterList []BootstrapChapter `json:"chapter_list"`
}

// BootstrapChapter 方案中的章节。
type BootstrapChapter struct {
	ChapterName string `json:"chapter_name"`
	SummaryText string `json:"summary_text"`
}

// BootstrapCharacter 方案中的角色卡。
type BootstrapCharacter struct {
	CharacterName   string `json:"character_name"`
	RoleTypeCode    string `json:"role_type_code"`
	PersonalityText string `json:"personality_text"`
	GoalText        string `json:"goal_text"`
	BackgroundText  string `json:"background_text"`
}

// BootstrapLore 方案中的世界观设定。
type BootstrapLore struct {
	LoreName     string `json:"lore_name"`
	LoreTypeCode string `json:"lore_type_code"`
	ContentText  string `json:"content_text"`
}

// bootstrapSystemPrompt 开书系统提示词：只输出严格 JSON，不写正文。
const bootstrapSystemPrompt = `你是长篇小说策划。把用户的一句创意变成一份可以直接开工的开书方案，只输出一个 JSON 对象，不输出解释、Markdown 围栏或任何多余文字。

方案要求：
- project_name：能一眼看出题材与卖点的书名，8-16 字。
- selling_point_text：一句话填满「一个什么人，因为什么事，必须做到什么，否则失去什么」。
- synopsis_text：200 字内简介，首句给钩子，末句给卖点。
- style_text：两三句文风与叙事约束（人称、节奏、对话密度、禁止项）。
- volume_list：1-2 卷；每卷写「从哪到哪 / 卷末翻什么牌」，并给出章节。
- chapter_list：每章写「发生什么 / 章末停在哪」；第一章必须在 300 字内出现冲突或异常。
- character_list：3-5 个角色，protagonist 至少一个；每个写欲望、性格、背景。
- lore_list：3-5 条世界观设定，优先写成「规则 / 边界 / 代价」三段的规则卡。

篇幅纪律（硬约束，超出会被截断导致整份方案作废）：
- 每章 summary_text 不超过 40 字，只写事件与钩子，不写描写。
- 每卷 summary_text 不超过 60 字。
- 每个角色每个字段不超过 40 字；每条设定 content_text 不超过 80 字。
- 简介不超过 150 字，文风不超过 60 字。

JSON 结构（字段名与层级必须完全一致，不要增删字段）：
{"project_name":"","genre_code":"","selling_point_text":"","synopsis_text":"","style_text":"",
"volume_list":[{"volume_name":"","summary_text":"","chapter_list":[{"chapter_name":"","summary_text":""}]}],
"character_list":[{"character_name":"","role_type_code":"protagonist","personality_text":"","goal_text":"","background_text":""}],
"lore_list":[{"lore_name":"","lore_type_code":"rule","content_text":""}]}`

// BootstrapPrompt 拼装开书用户提示词（创意 + 题材 + 章节规模）。
func BootstrapPrompt(ideaText, genreCode string, chapterCount int) string {
	var builder strings.Builder
	builder.WriteString("创意：")
	builder.WriteString(strings.TrimSpace(ideaText))
	if strings.TrimSpace(genreCode) != "" {
		builder.WriteString("\n题材编码：")
		builder.WriteString(strings.TrimSpace(genreCode))
	}
	builder.WriteString(fmt.Sprintf("\n章节规模：全书约 %d 章，按卷均分。", chapterCount))
	builder.WriteString("\n只输出 JSON。")
	return builder.String()
}

// GenerateBootstrapPlan 调用对话模型生成开书方案。
// 走流式累积：结构化长输出的非流式调用会长时间无响应头，容易被网关首包超时打断。
func (c *Client) GenerateBootstrapPlan(ctx context.Context, spec ProviderSpec, ideaText, genreCode string, chapterCount int) (BootstrapPlan, error) {
	stream, err := c.Stream(ctx, spec, []*schema.Message{
		{Role: schema.System, Content: bootstrapSystemPrompt},
		{Role: schema.User, Content: BootstrapPrompt(ideaText, genreCode, chapterCount)},
	})
	if err != nil {
		return BootstrapPlan{}, fmt.Errorf("bootstrap stream: %w", err)
	}
	defer stream.Close()
	var builder strings.Builder
	var lastChunk *schema.Message
	for {
		chunk, recvErr := stream.Recv()
		if recvErr == io.EOF {
			break
		}
		if recvErr != nil {
			return BootstrapPlan{}, fmt.Errorf("bootstrap stream recv: %w", recvErr)
		}
		lastChunk = chunk
		builder.WriteString(chunk.Content)
	}
	c.RecordStreamUsage(spec, usageOf(lastChunk), 0)
	return ParseBootstrapPlan(builder.String())
}

// ParseBootstrapPlan 从模型回复中解析开书方案：容忍代码围栏，要求 JSON 对象且至少一卷。
func ParseBootstrapPlan(content string) (BootstrapPlan, error) {
	trimmed := strings.TrimSpace(content)
	start := strings.Index(trimmed, "{")
	end := strings.LastIndex(trimmed, "}")
	if start < 0 || end <= start {
		return BootstrapPlan{}, fmt.Errorf("开书方案缺少 JSON 对象")
	}
	var plan BootstrapPlan
	if err := json.Unmarshal([]byte(trimmed[start:end+1]), &plan); err != nil {
		return BootstrapPlan{}, fmt.Errorf("开书方案 JSON 解析失败: %w", err)
	}
	if strings.TrimSpace(plan.ProjectName) == "" {
		return BootstrapPlan{}, fmt.Errorf("开书方案缺少书名")
	}
	if len(plan.VolumeList) == 0 {
		return BootstrapPlan{}, fmt.Errorf("开书方案缺少卷章结构")
	}
	return plan, nil
}
