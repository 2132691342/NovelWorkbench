// 本文件把工具资产转成模型可见的工具声明（eino ToolInfo），并提供参数解码。
// 工具由模型自行决定是否调用：会话里只下发「有哪些工具」，不预判、不在命令里强制注入。

package ai

import (
	"encoding/json"
	"strconv"
	"strings"

	"github.com/cloudwego/eino/schema"
)

// toolParamDescRunes 参数说明截断长度（过长会让工具声明挤占上下文预算）。
const toolParamDescRunes = 120

// ToolSpec 一次对话可下发的工具声明（service 由工具资产转换而来，ai 不依赖仓储）。
type ToolSpec struct {
	ToolCode        string      // 工具编码（即模型调用的 function name）
	ToolName        string      // 工具名称
	DescriptionText string      // 一句话说明（模型据此判断何时调用）
	ParamList       []ToolParam // 参数声明
}

// toToolInfo 构造 eino 工具声明。
// function name 用工具编码而不是中文名：模型按名字回填调用，编码是稳定的机器标识。
func (s ToolSpec) toToolInfo() *schema.ToolInfo {
	params := make(map[string]*schema.ParameterInfo, len(s.ParamList))
	for i := range s.ParamList {
		param := &s.ParamList[i]
		desc := param.DescriptionText
		if strings.TrimSpace(desc) == "" {
			desc = param.ParamLabel
		}
		params[param.ParamName] = &schema.ParameterInfo{
			Type:     schema.String,
			Desc:     truncateRunes(desc, toolParamDescRunes),
			Required: param.RequiredFlag,
		}
	}
	info := &schema.ToolInfo{
		Name: s.ToolCode,
		Desc: s.ToolName + "：" + s.DescriptionText,
	}
	if len(params) > 0 {
		info.ParamsOneOf = schema.NewParamsOneOfByParams(params)
	}
	return info
}

// ToolInfos 把工具规格列表转成模型声明列表（空列表返回 nil，调用方据此跳过绑定）。
func ToolInfos(specList []ToolSpec) []*schema.ToolInfo {
	if len(specList) == 0 {
		return nil
	}
	infoList := make([]*schema.ToolInfo, 0, len(specList))
	for i := range specList {
		infoList = append(infoList, specList[i].toToolInfo())
	}
	return infoList
}

// DecodeToolArguments 解析模型给出的工具参数 JSON（空串或非法 JSON 返回空表，不阻断）。
// 参数一律按字符串处理：工具参数本就用于表达「正文/检索词/句数」这类标量。
func DecodeToolArguments(argumentsJSON string) map[string]string {
	paramValue := make(map[string]string, 4)
	trimmed := strings.TrimSpace(argumentsJSON)
	if trimmed == "" {
		return paramValue
	}
	var raw map[string]any
	if err := json.Unmarshal([]byte(trimmed), &raw); err != nil {
		return paramValue
	}
	for key, value := range raw {
		switch typed := value.(type) {
		case string:
			paramValue[key] = typed
		case float64:
			paramValue[key] = strconv.FormatFloat(typed, 'f', -1, 64)
		case bool:
			paramValue[key] = strconv.FormatBool(typed)
		default:
			continue // 嵌套结构工具用不到，忽略而不是报错
		}
	}
	return paramValue
}
