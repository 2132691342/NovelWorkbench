package service

import (
	"context"
	"strings"

	"github.com/2132691342/NovelWorkbench/backend/internal/ai"
	"github.com/2132691342/NovelWorkbench/backend/internal/domain"
	"github.com/2132691342/NovelWorkbench/backend/internal/pkg"
	"github.com/2132691342/NovelWorkbench/backend/internal/repo"
)

// SlashService 斜杠命令业务：内置技能播种、命令 CRUD 与正文解析。
type SlashService struct {
	slashRepo *repo.SlashRepo
}

// NewSlashService 构造斜杠命令服务。
func NewSlashService(slashRepo *repo.SlashRepo) *SlashService {
	return &SlashService{slashRepo: slashRepo}
}

// SeedBuiltin 播种内置技能（幂等：按 command_code 查缺补插，builtin_flag=true）。
func (s *SlashService) SeedBuiltin(ctx context.Context) error {
	for _, skill := range ai.BuiltinSkills() {
		existing, err := s.slashRepo.GetByCode(ctx, skill.CommandCode)
		if err != nil {
			return err
		}
		if existing != nil {
			continue // 已存在不覆盖，保留用户对提示词的定制
		}
		do := &domain.SlashCommandDO{
			CommandULID:     pkg.NewULID(domain.IDPrefixSlashCommand),
			CommandCode:     skill.CommandCode,
			CommandName:     skill.CommandName,
			CategoryCode:    skill.CategoryCode,
			DescriptionText: skill.DescriptionText,
			PromptTemplate:  skill.PromptTemplate,
			EnabledFlag:     true,
			BuiltinFlag:     true,
			SortOrder:       skill.SortOrder,
		}
		if err := s.slashRepo.Create(ctx, do); err != nil {
			return err
		}
	}
	return nil
}

// List 查询斜杠命令列表（可选仅启用）。
func (s *SlashService) List(ctx context.Context, req *domain.SlashListREQ) ([]domain.SlashCommandVO, error) {
	dos, err := s.slashRepo.List(ctx, req.EnabledOnly)
	if err != nil {
		return nil, err
	}
	vos := make([]domain.SlashCommandVO, 0, len(dos))
	for i := range dos {
		vos = append(vos, toSlashCommandVO(&dos[i]))
	}
	return vos, nil
}

// Save 新建/更新斜杠命令（有 command_ulid 为更新；builtin_flag 不可变更，故不入更新列）。
func (s *SlashService) Save(ctx context.Context, req *domain.SlashCommandREQ) (domain.SlashCommandVO, error) {
	if req.CommandULID == "" {
		existing, err := s.slashRepo.GetByCode(ctx, req.CommandCode)
		if err != nil {
			return domain.SlashCommandVO{}, err
		}
		if existing != nil {
			return domain.SlashCommandVO{}, pkg.NewBizError(domain.RespCodeConflict, "命令编码已存在")
		}
		enabledFlag := true
		if req.EnabledFlag != nil {
			enabledFlag = *req.EnabledFlag
		}
		do := &domain.SlashCommandDO{
			CommandULID:     pkg.NewULID(domain.IDPrefixSlashCommand),
			CommandCode:     req.CommandCode,
			CommandName:     req.CommandName,
			CategoryCode:    req.CategoryCode,
			DescriptionText: req.DescriptionText,
			PromptTemplate:  req.PromptTemplate,
			EnabledFlag:     enabledFlag,
			BuiltinFlag:     false,
			SortOrder:       req.SortOrder,
		}
		if err := s.slashRepo.Create(ctx, do); err != nil {
			return domain.SlashCommandVO{}, err
		}
		return toSlashCommandVO(do), nil
	}
	existing, err := s.slashRepo.GetByULID(ctx, req.CommandULID)
	if err != nil {
		return domain.SlashCommandVO{}, err
	}
	if existing == nil {
		return domain.SlashCommandVO{}, pkg.NewBizError(domain.RespCodeNotFound, "命令不存在")
	}
	codeConflict, err := s.slashRepo.GetByCode(ctx, req.CommandCode)
	if err != nil {
		return domain.SlashCommandVO{}, err
	}
	if codeConflict != nil && codeConflict.CommandULID != req.CommandULID {
		return domain.SlashCommandVO{}, pkg.NewBizError(domain.RespCodeConflict, "命令编码已存在")
	}
	fields := map[string]any{
		"command_code":     req.CommandCode,
		"command_name":     req.CommandName,
		"prompt_template":  req.PromptTemplate,
		"description_text": req.DescriptionText,
		"sort_order":       req.SortOrder,
	}
	if req.CategoryCode != "" {
		fields["category_code"] = req.CategoryCode
	}
	if req.EnabledFlag != nil {
		fields["enabled_flag"] = *req.EnabledFlag
	}
	if err := s.slashRepo.Update(ctx, req.CommandULID, fields); err != nil {
		return domain.SlashCommandVO{}, err
	}
	updated, err := s.slashRepo.GetByULID(ctx, req.CommandULID)
	if err != nil {
		return domain.SlashCommandVO{}, err
	}
	return toSlashCommandVO(updated), nil
}

// Delete 删除斜杠命令（内置不可删）。
func (s *SlashService) Delete(ctx context.Context, req *domain.SlashDeleteREQ) error {
	existing, err := s.slashRepo.GetByULID(ctx, req.CommandULID)
	if err != nil {
		return err
	}
	if existing == nil {
		return pkg.NewBizError(domain.RespCodeNotFound, "命令不存在")
	}
	if existing.BuiltinFlag {
		return pkg.NewBizError(domain.RespCodeBizFailed, "内置命令不可删除")
	}
	return s.slashRepo.Delete(ctx, req.CommandULID)
}

// Parse 解析正文首 token 斜杠命令（命中启用命令返回技能编码与剩余正文）。
func (s *SlashService) Parse(ctx context.Context, req *domain.SlashParseREQ) (domain.SlashParseDTO, error) {
	commands, err := s.slashRepo.List(ctx, true)
	if err != nil {
		return domain.SlashParseDTO{}, err
	}
	return parseSlashContent(req.ContentText, commands), nil
}

// parseSlashContent 纯解析逻辑：首 token 形如 /code 且命中启用命令时剥离命令与参数，
// 未命中时正文原样保留（skill_code 为空，placeholder_map 为空表）。
func parseSlashContent(contentText string, commands []domain.SlashCommandDO) domain.SlashParseDTO {
	result := domain.SlashParseDTO{PlaceholderMap: map[string]string{}}
	trimmed := strings.TrimSpace(contentText)
	if trimmed == "" || !strings.HasPrefix(trimmed, "/") {
		result.RemainderText = contentText
		return result
	}
	tokenEnd := strings.IndexAny(trimmed, " \t\r\n")
	commandCode, remainderText := "", ""
	if tokenEnd < 0 {
		commandCode = trimmed[1:]
	} else {
		commandCode = trimmed[1:tokenEnd]
		remainderText = strings.TrimSpace(trimmed[tokenEnd:])
	}
	for i := range commands {
		if commands[i].EnabledFlag && commands[i].CommandCode == commandCode {
			result.SkillCode = commands[i].CommandCode
			result.RemainderText = remainderText
			result.PlaceholderMap["user_input"] = remainderText
			return result
		}
	}
	result.RemainderText = contentText
	return result
}

// toSlashCommandVO 组装斜杠命令视图对象。
func toSlashCommandVO(do *domain.SlashCommandDO) domain.SlashCommandVO {
	return domain.SlashCommandVO{
		CommandULID:     do.CommandULID,
		CommandCode:     do.CommandCode,
		CommandName:     do.CommandName,
		CategoryCode:    do.CategoryCode,
		DescriptionText: do.DescriptionText,
		PromptTemplate:  do.PromptTemplate,
		EnabledFlag:     do.EnabledFlag,
		BuiltinFlag:     do.BuiltinFlag,
		SortOrder:       do.SortOrder,
	}
}
