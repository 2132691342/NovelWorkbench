// 本文件实现技能业务：内置技能播种、全局/项目作用域合并、模板渲染与参考资料命中。

package service

import (
	"context"
	"encoding/json"
	"sort"
	"strings"

	"github.com/2132691342/NovelWorkbench/backend/internal/ai"
	"github.com/2132691342/NovelWorkbench/backend/internal/domain"
	"github.com/2132691342/NovelWorkbench/backend/internal/pkg"
	"github.com/2132691342/NovelWorkbench/backend/internal/repo"
)

// 技能参考资料的注入上限（字符）：命中资料超过后按排序号截断，防上下文失控。
const skillReferenceBudgetChars = 4000

// skillRefKeywordSeparator 参考资料关键词分隔符。
const skillRefKeywordSeparator = ","

// SkillService 技能业务：播种、CRUD、作用域合并解析、模板渲染与参考资料管理。
type SkillService struct {
	skillRepo   *repo.SkillRepo   // 技能仓储
	projectRepo *repo.ProjectRepo // 项目仓储（项目技能归属校验）
}

// NewSkillService 构造技能服务。
func NewSkillService(skillRepo *repo.SkillRepo, projectRepo *repo.ProjectRepo) *SkillService {
	return &SkillService{skillRepo: skillRepo, projectRepo: projectRepo}
}

// SeedDefaults 播种内置全局技能（幂等：按编码查缺补插；已存在则保留作者的改动）。
func (s *SkillService) SeedDefaults(ctx context.Context) error {
	for _, builtin := range ai.BuiltinSkills() {
		existing, err := s.skillRepo.GetByCode(ctx, domain.SkillScopeGlobal, "", builtin.CommandCode)
		if err != nil {
			return err
		}
		if existing != nil {
			continue
		}
		do := &domain.SkillDO{
			SkillULID:        pkg.NewULID(domain.IDPrefixSkill),
			SkillCode:        builtin.CommandCode,
			SkillName:        builtin.CommandName,
			ScopeCode:        domain.SkillScopeGlobal,
			CategoryCode:     builtin.CategoryCode,
			DescriptionText:  builtin.DescriptionText,
			PromptTemplate:   builtin.PromptTemplate,
			VariableSpecJSON: "[]",
			EnabledFlag:      true,
			BuiltinFlag:      true,
			SortOrder:        builtin.SortOrder,
		}
		if err := s.skillRepo.Create(ctx, do); err != nil {
			return err
		}
	}
	return nil
}

// List 查询技能列表：projectULID 非空时合并「项目技能 + 全局技能」并标记被覆盖项。
func (s *SkillService) List(ctx context.Context, req *domain.SkillListREQ) ([]domain.SkillVO, error) {
	if req.ProjectULID != "" {
		if err := s.mustProjectAccess(ctx, req.ProjectULID); err != nil {
			return nil, err
		}
	}
	dos, err := s.skillRepo.List(ctx, req)
	if err != nil {
		return nil, err
	}
	skillULIDs := make([]string, 0, len(dos))
	for i := range dos {
		skillULIDs = append(skillULIDs, dos[i].SkillULID)
	}
	refCounts, err := s.skillRepo.CountReferences(ctx, skillULIDs)
	if err != nil {
		return nil, err
	}
	vos := make([]domain.SkillVO, 0, len(dos))
	for i := range dos {
		vo := toSkillVO(&dos[i], refCounts[dos[i].SkillULID])
		vo.OverriddenFlag = isOverriddenGlobalSkill(&dos[i], dos)
		vos = append(vos, vo)
	}
	return vos, nil
}

// isOverriddenGlobalSkill 判断全局技能是否被同编码的启用项目技能覆盖（纯函数）。
func isOverriddenGlobalSkill(target *domain.SkillDO, all []domain.SkillDO) bool {
	if target.ScopeCode != domain.SkillScopeGlobal {
		return false
	}
	for i := range all {
		if all[i].ScopeCode == domain.SkillScopeProject && all[i].EnabledFlag && all[i].SkillCode == target.SkillCode {
			return true
		}
	}
	return false
}

// Detail 查询技能详情。
func (s *SkillService) Detail(ctx context.Context, skillULID string) (domain.SkillVO, error) {
	do, err := s.skillRepo.GetByULID(ctx, skillULID)
	if err != nil {
		return domain.SkillVO{}, err
	}
	if do == nil {
		return domain.SkillVO{}, pkg.NewBizError(domain.RespCodeNotFound, "技能不存在")
	}
	refCounts, err := s.skillRepo.CountReferences(ctx, []string{do.SkillULID})
	if err != nil {
		return domain.SkillVO{}, err
	}
	return toSkillVO(do, refCounts[do.SkillULID]), nil
}

// Save 创建或更新技能（同作用域下编码唯一；项目技能必须带项目且校验归属）。
func (s *SkillService) Save(ctx context.Context, req *domain.SkillREQ) (domain.SkillVO, error) {
	scopeCode := req.ScopeCode
	if scopeCode == "" {
		scopeCode = domain.SkillScopeGlobal
	}
	projectULID := ""
	if scopeCode == domain.SkillScopeProject {
		if req.ProjectULID == "" {
			return domain.SkillVO{}, pkg.NewBizError(domain.RespCodeInvalidParam, "项目技能必须指定项目")
		}
		if err := s.mustProjectAccess(ctx, req.ProjectULID); err != nil {
			return domain.SkillVO{}, err
		}
		projectULID = req.ProjectULID
	}
	if err := validateSkillTemplate(req.PromptTemplate, req.VariableList); err != nil {
		return domain.SkillVO{}, err
	}
	variableJSON, err := marshalSkillVariables(req.VariableList)
	if err != nil {
		return domain.SkillVO{}, err
	}
	categoryCode := req.CategoryCode
	if categoryCode == "" {
		categoryCode = domain.SkillCategoryOther
	}
	existingByCode, err := s.skillRepo.GetByCode(ctx, scopeCode, projectULID, req.SkillCode)
	if err != nil {
		return domain.SkillVO{}, err
	}
	if existingByCode != nil && existingByCode.SkillULID != req.SkillULID {
		return domain.SkillVO{}, pkg.NewBizError(domain.RespCodeConflict, "同作用域下技能编码已存在")
	}
	enabledFlag := true
	if req.EnabledFlag != nil {
		enabledFlag = *req.EnabledFlag
	}
	if req.SkillULID == "" {
		do := &domain.SkillDO{
			SkillULID:        pkg.NewULID(domain.IDPrefixSkill),
			SkillCode:        req.SkillCode,
			SkillName:        req.SkillName,
			ScopeCode:        scopeCode,
			ProjectULID:      projectULID,
			CategoryCode:     categoryCode,
			DescriptionText:  req.DescriptionText,
			PromptTemplate:   req.PromptTemplate,
			VariableSpecJSON: variableJSON,
			EnabledFlag:      enabledFlag,
			SortOrder:        req.SortOrder,
		}
		if err := s.skillRepo.Create(ctx, do); err != nil {
			return domain.SkillVO{}, err
		}
		return toSkillVO(do, 0), nil
	}
	current, err := s.skillRepo.GetByULID(ctx, req.SkillULID)
	if err != nil {
		return domain.SkillVO{}, err
	}
	if current == nil {
		return domain.SkillVO{}, pkg.NewBizError(domain.RespCodeNotFound, "技能不存在")
	}
	if current.BuiltinFlag && (current.SkillCode != req.SkillCode || current.ScopeCode != scopeCode) {
		return domain.SkillVO{}, pkg.NewBizError(domain.RespCodeBizFailed, "内置技能不允许修改编码与作用域")
	}
	fields := map[string]any{
		"skill_code":         req.SkillCode,
		"skill_name":         req.SkillName,
		"scope_code":         scopeCode,
		"project_ulid":       projectULID,
		"category_code":      categoryCode,
		"description_text":   req.DescriptionText,
		"prompt_template":    req.PromptTemplate,
		"variable_spec_json": variableJSON,
		"enabled_flag":       enabledFlag,
		"sort_order":         req.SortOrder,
	}
	if err := s.skillRepo.Update(ctx, req.SkillULID, fields); err != nil {
		return domain.SkillVO{}, err
	}
	current.SkillCode = req.SkillCode
	current.SkillName = req.SkillName
	current.ScopeCode = scopeCode
	current.ProjectULID = projectULID
	current.CategoryCode = categoryCode
	current.DescriptionText = req.DescriptionText
	current.PromptTemplate = req.PromptTemplate
	current.VariableSpecJSON = variableJSON
	current.EnabledFlag = enabledFlag
	current.SortOrder = req.SortOrder
	refCounts, err := s.skillRepo.CountReferences(ctx, []string{current.SkillULID})
	if err != nil {
		return domain.SkillVO{}, err
	}
	return toSkillVO(current, refCounts[current.SkillULID]), nil
}

// Delete 删除技能（内置不可删，参考资料级联清理）。
func (s *SkillService) Delete(ctx context.Context, req *domain.SkillDeleteREQ) error {
	current, err := s.skillRepo.GetByULID(ctx, req.SkillULID)
	if err != nil {
		return err
	}
	if current == nil {
		return pkg.NewBizError(domain.RespCodeNotFound, "技能不存在")
	}
	if current.BuiltinFlag {
		return pkg.NewBizError(domain.RespCodeBizFailed, "内置技能不允许删除，可停用")
	}
	if err := s.skillRepo.DeleteReferencesBySkill(ctx, req.SkillULID); err != nil {
		return err
	}
	return s.skillRepo.Delete(ctx, req.SkillULID)
}

// Resolve 解析用户输入为技能与变量：显式 skill_code 优先，否则解析正文首 token 的 /code。
func (s *SkillService) Resolve(ctx context.Context, req *domain.SkillResolveREQ) (domain.SkillResolveDTO, error) {
	if err := s.mustProjectAccess(ctx, req.ProjectULID); err != nil {
		return domain.SkillResolveDTO{}, err
	}
	skillCode, remainderText := resolveSkillCode(req.SkillCode, req.ContentText)
	if skillCode == "" {
		return domain.SkillResolveDTO{RemainderText: remainderText, VariableMap: map[string]string{}, MissingList: []string{}, RefTextList: []string{}}, nil
	}
	skill, err := s.resolveByCode(ctx, req.ProjectULID, skillCode)
	if err != nil {
		return domain.SkillResolveDTO{}, err
	}
	variableMap, missingList := resolveSkillVariables(skill.SkillCode, skill.VariableList, req.VariableValue, remainderText)
	refList, err := s.matchReferences(ctx, skill, req.ContentText, remainderText)
	if err != nil {
		return domain.SkillResolveDTO{}, err
	}
	return domain.SkillResolveDTO{
		SkillULID:     skill.SkillULID,
		SkillCode:     skill.SkillCode,
		SkillName:     skill.SkillName,
		RemainderText: remainderText,
		VariableMap:   variableMap,
		MissingList:   missingList,
		RefTextList:   refList,
	}, nil
}

// SkillTemplate 返回启用技能的提示词模板（对话上下文装配需要 {{context}} 等占位符保留原样）。
func (s *SkillService) SkillTemplate(ctx context.Context, projectULID, skillCode string) (domain.SkillVO, error) {
	return s.resolveByCode(ctx, projectULID, skillCode)
}

// resolveByCode 按 (项目, 编码) 解析启用中的技能：项目技能优先于全局技能。
func (s *SkillService) resolveByCode(ctx context.Context, projectULID, skillCode string) (domain.SkillVO, error) {
	var projectSkill *domain.SkillDO
	if projectULID != "" {
		found, err := s.skillRepo.GetByCode(ctx, domain.SkillScopeProject, projectULID, skillCode)
		if err != nil {
			return domain.SkillVO{}, err
		}
		projectSkill = found
	}
	globalSkill, err := s.skillRepo.GetByCode(ctx, domain.SkillScopeGlobal, "", skillCode)
	if err != nil {
		return domain.SkillVO{}, err
	}
	winner := pickSkillByScope(projectSkill, globalSkill)
	if winner == nil {
		return domain.SkillVO{}, pkg.NewBizError(domain.RespCodeBizFailed, "技能不存在或已停用: "+skillCode)
	}
	return toSkillVO(winner, 0), nil
}

// pickSkillByScope 按作用域优先级挑选技能（纯函数）：
// 启用中的项目技能优先，其次启用中的全局技能；停用与不存在一律不命中。
func pickSkillByScope(projectSkill, globalSkill *domain.SkillDO) *domain.SkillDO {
	if projectSkill != nil && projectSkill.EnabledFlag {
		return projectSkill
	}
	if globalSkill != nil && globalSkill.EnabledFlag {
		return globalSkill
	}
	return nil
}

// mustProjectAccess 校验项目存在（技能作用域隔离用；不校验创建者，方便团队协作）。
func (s *SkillService) mustProjectAccess(ctx context.Context, projectULID string) error {
	project, err := s.projectRepo.GetByULID(ctx, projectULID)
	if err != nil {
		return err
	}
	if project == nil {
		return pkg.NewBizError(domain.RespCodeNotFound, "项目不存在")
	}
	return nil
}

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
	known := map[string]bool{"user_input": true, "chapter_content": true, "context": true, "skill_code": true}
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

// toSkillVO 组装技能视图对象。
func toSkillVO(do *domain.SkillDO, refCount int) domain.SkillVO {
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
		SortOrder:       do.SortOrder,
		CreatedAtMs:     do.CreatedAt.UnixMilli(),
		UpdatedAtMs:     do.UpdatedAt.UnixMilli(),
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
		CreatedAtMs: do.CreatedAt.UnixMilli(),
	}
}
