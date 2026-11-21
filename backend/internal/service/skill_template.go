// 本文件实现技能解析与模板工具的纯函数：斜杠命令解析、变量取值、占位符校验与视图组装。

package service

import (
	"encoding/json"
	"sort"
	"strings"

	"github.com/2132691342/NovelWorkbench/backend/internal/ai"
	"github.com/2132691342/NovelWorkbench/backend/internal/domain"
	"github.com/2132691342/NovelWorkbench/backend/internal/pkg"
)

// resolveSkillCode 解析技能编码与剩余正文（纯函数）：
// 显式传入的 skill_code 优先；否则解析正文首 token 的 /code，命中才剥离命令；
// 未命中技能时正文原样返回。
func resolveSkillCode(explicitCode, contentText string) (string, string) {
	if explicitCode != "" {
		return explicitCode, contentText
	}
	parsedCode, parsedRemainder := parseSlashToken(contentText)
	if parsedCode == "" {
		return "", contentText
	}
	return parsedCode, parsedRemainder
}

// parseSlashToken 解析正文首 token 的 /code：返回编码与去掉命令后的正文。
func parseSlashToken(contentText string) (string, string) {
	trimmed := strings.TrimSpace(contentText)
	if !strings.HasPrefix(trimmed, "/") {
		return "", trimmed
	}
	end := strings.IndexAny(trimmed, " \t\r\n")
	if end < 0 {
		return "", ""
	}
	code := strings.TrimPrefix(trimmed[:end], "/")
	if code == "" {
		return "", trimmed
	}
	return code, strings.TrimSpace(trimmed[end:])
}

// resolveSkillVariables 计算变量取值：显式传值 > 声明缺省值 > 必填缺失记入 missing。
func resolveSkillVariables(skillCode string, variableList []domain.SkillVariableDO,
	valueMap map[string]string, userInput string) (map[string]string, []string) {
	resolved := map[string]string{"skill_code": skillCode, "user_input": userInput}
	missing := make([]string, 0)
	for i := range variableList {
		name := variableList[i].VariableName
		if name == "" {
			continue
		}
		if value, ok := valueMap[name]; ok && value != "" {
			resolved[name] = value
			continue
		}
		if variableList[i].DefaultValue != "" {
			resolved[name] = variableList[i].DefaultValue
			continue
		}
		if variableList[i].RequiredFlag {
			missing = append(missing, name)
			continue
		}
		resolved[name] = ""
	}
	sort.Strings(missing)
	return resolved, missing
}

// validateSkillTemplate 校验模板中的占位符都已在变量声明中登记（内置占位符除外）。
func validateSkillTemplate(promptTemplate string, variableList []domain.SkillVariableDO) error {
	known := make(map[string]bool, len(ai.BuiltinPlaceholders)+len(variableList))
	for name := range ai.BuiltinPlaceholders {
		known[name] = true
	}
	for i := range variableList {
		if variableList[i].VariableName == "" {
			return pkg.NewBizError(domain.RespCodeInvalidParam, "变量名不能为空")
		}
		known[variableList[i].VariableName] = true
	}
	unknown := ai.TemplatePlaceholders(promptTemplate, known)
	if len(unknown) > 0 {
		return pkg.NewBizError(domain.RespCodeInvalidParam,
			"模板中的占位符未声明变量: "+strings.Join(unknown, skillRefKeywordSeparator))
	}
	return nil
}

// marshalSkillVariables 序列化变量声明（空列表存 "[]"，保证列非 NULL）。
func marshalSkillVariables(variableList []domain.SkillVariableDO) (string, error) {
	if len(variableList) == 0 {
		return "[]", nil
	}
	data, err := json.Marshal(variableList)
	if err != nil {
		return "", pkg.WrapBizError(domain.RespCodeInvalidParam, "变量声明序列化失败", err)
	}
	return string(data), nil
}

// unmarshalSkillVariables 反序列化变量声明（损坏数据按空列表处理）。
func unmarshalSkillVariables(variableSpecJSON string) []domain.SkillVariableDO {
	variableList := make([]domain.SkillVariableDO, 0, 4)
	if variableSpecJSON == "" {
		return variableList
	}
	if err := json.Unmarshal([]byte(variableSpecJSON), &variableList); err != nil {
		return []domain.SkillVariableDO{}
	}
	return variableList
}

// toSkillVO 组装技能视图对象（内置技能带上所属流程域，便于管理端按域归档）。
func toSkillVO(do *domain.SkillDO, refCount int) domain.SkillVO {
	domainCode, domainName := "", ""
	if do.BuiltinFlag {
		domainCode, domainName = ai.BuiltinSkillDomainOf(do.SkillCode)
	}
	return domain.SkillVO{
		SkillULID:       do.SkillULID,
		SkillCode:       do.SkillCode,
		SkillName:       do.SkillName,
		ScopeCode:       do.ScopeCode,
		ProjectULID:     do.ProjectULID,
		CategoryCode:    do.CategoryCode,
		DescriptionText: do.DescriptionText,
		PromptTemplate:  do.PromptTemplate,
		VariableList:    unmarshalSkillVariables(do.VariableSpecJSON),
		RefCount:        refCount,
		EnabledFlag:     do.EnabledFlag,
		BuiltinFlag:     do.BuiltinFlag,
		CustomizedFlag:  do.CustomizedFlag,
		DomainCode:      domainCode,
		DomainName:      domainName,
		SortOrder:       do.SortOrder,
		CreatedAt:       pkg.FormatTime(do.CreatedAt),
		UpdatedAt:       pkg.FormatTime(do.UpdatedAt),
	}
}

// toSkillReferenceVO 组装参考资料视图对象。
func toSkillReferenceVO(do *domain.SkillReferenceDO) domain.SkillReferenceVO {
	return domain.SkillReferenceVO{
		RefULID:     do.RefULID,
		SkillULID:   do.SkillULID,
		RefName:     do.RefName,
		RefText:     do.RefText,
		KeywordList: do.KeywordList,
		SortOrder:   do.SortOrder,
		CreatedAt:   pkg.FormatTime(do.CreatedAt),
	}
}
