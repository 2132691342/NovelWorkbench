// 本文件实现技能参考资料的存取与关键词命中（技能命中时按关键词挑片段注入上下文）。

package service

import (
	"context"
	"strings"

	"github.com/2132691342/NovelWorkbench/backend/internal/domain"
	"github.com/2132691342/NovelWorkbench/backend/internal/pkg"
)

// SaveReference 创建或更新技能参考资料。
func (s *SkillService) SaveReference(ctx context.Context, req *domain.SkillReferenceREQ) (domain.SkillReferenceVO, error) {
	skill, err := s.skillRepo.GetByULID(ctx, req.SkillULID)
	if err != nil {
		return domain.SkillReferenceVO{}, err
	}
	if skill == nil {
		return domain.SkillReferenceVO{}, pkg.NewBizError(domain.RespCodeNotFound, "技能不存在")
	}
	if req.RefULID == "" {
		do := &domain.SkillReferenceDO{
			RefULID:     pkg.NewULID(domain.IDPrefixSkillReference),
			SkillULID:   req.SkillULID,
			RefName:     req.RefName,
			RefText:     req.RefText,
			KeywordList: normalizeKeywordList(req.KeywordList),
			SortOrder:   req.SortOrder,
		}
		if err := s.skillRepo.CreateReference(ctx, do); err != nil {
			return domain.SkillReferenceVO{}, err
		}
		return toSkillReferenceVO(do), nil
	}
	current, err := s.skillRepo.GetReference(ctx, req.RefULID)
	if err != nil {
		return domain.SkillReferenceVO{}, err
	}
	if current == nil {
		return domain.SkillReferenceVO{}, pkg.NewBizError(domain.RespCodeNotFound, "参考资料不存在")
	}
	if current.SkillULID != req.SkillULID {
		return domain.SkillReferenceVO{}, pkg.NewBizError(domain.RespCodeInvalidParam, "参考资料不属于该技能")
	}
	fields := map[string]any{
		"ref_name":     req.RefName,
		"ref_text":     req.RefText,
		"keyword_list": normalizeKeywordList(req.KeywordList),
		"sort_order":   req.SortOrder,
	}
	if err := s.skillRepo.UpdateReference(ctx, req.RefULID, fields); err != nil {
		return domain.SkillReferenceVO{}, err
	}
	current.RefName = req.RefName
	current.RefText = req.RefText
	current.KeywordList = normalizeKeywordList(req.KeywordList)
	current.SortOrder = req.SortOrder
	return toSkillReferenceVO(current), nil
}

// DeleteReference 删除技能参考资料。
func (s *SkillService) DeleteReference(ctx context.Context, req *domain.SkillReferenceDeleteREQ) error {
	current, err := s.skillRepo.GetReference(ctx, req.RefULID)
	if err != nil {
		return err
	}
	if current == nil {
		return pkg.NewBizError(domain.RespCodeNotFound, "参考资料不存在")
	}
	return s.skillRepo.DeleteReference(ctx, req.RefULID)
}

// ReferenceList 查询技能的参考资料列表。
func (s *SkillService) ReferenceList(ctx context.Context, req *domain.SkillListBySkillREQ) ([]domain.SkillReferenceVO, error) {
	dos, err := s.skillRepo.ListReferences(ctx, req.SkillULID)
	if err != nil {
		return nil, err
	}
	vos := make([]domain.SkillReferenceVO, 0, len(dos))
	for i := range dos {
		vos = append(vos, toSkillReferenceVO(&dos[i]))
	}
	return vos, nil
}

// matchReferences 命中参考资料：关键词为空总是注入，命中关键词才注入，整体受字符预算约束。
func (s *SkillService) matchReferences(ctx context.Context, skill domain.SkillVO, haystacks ...string) ([]string, error) {
	dos, err := s.skillRepo.ListReferences(ctx, skill.SkillULID)
	if err != nil {
		return nil, err
	}
	return selectSkillReferences(dos, haystacks, skillReferenceBudgetChars), nil
}

// selectSkillReferences 挑选要注入的资料片段（纯函数）：
// 关键词命中才入选；正文为空的资料跳过；累计字符超出预算即停止（后续资料一律不取）。
func selectSkillReferences(dos []domain.SkillReferenceDO, haystacks []string, budgetChars int) []string {
	result := make([]string, 0, len(dos))
	usedChars := 0
	for i := range dos {
		if !matchSkillReferenceKeywords(dos[i].KeywordList, haystacks) {
			continue
		}
		text := strings.TrimSpace(dos[i].RefText)
		if text == "" {
			continue
		}
		if usedChars+len(text) > budgetChars {
			break
		}
		usedChars += len(text)
		result = append(result, "【"+dos[i].RefName+"】\n"+text)
	}
	return result
}

// matchSkillReferenceKeywords 判断参考资料是否命中：关键词为空总是命中，否则任一关键词出现在文本中即命中。
func matchSkillReferenceKeywords(keywordList string, haystacks []string) bool {
	keywords := splitKeywordList(keywordList)
	if len(keywords) == 0 {
		return true
	}
	for _, keyword := range keywords {
		for _, haystack := range haystacks {
			if strings.Contains(haystack, keyword) {
				return true
			}
		}
	}
	return false
}

// splitKeywordList 拆分逗号分隔的关键词（去空去重）。
func splitKeywordList(keywordList string) []string {
	result := make([]string, 0, 4)
	seen := make(map[string]bool, 4)
	for _, item := range strings.Split(keywordList, skillRefKeywordSeparator) {
		keyword := strings.TrimSpace(item)
		if keyword == "" || seen[keyword] {
			continue
		}
		seen[keyword] = true
		result = append(result, keyword)
	}
	return result
}

// normalizeKeywordList 归一化关键词列表（去重后重新以逗号拼接）。
func normalizeKeywordList(keywordList string) string {
	return strings.Join(splitKeywordList(keywordList), skillRefKeywordSeparator)
}
