// 本文件实现应用配置业务：默认配置播种、白名单查询与更新。

package service

import (
	"context"
	"strconv"

	"github.com/2132691342/NovelWorkbench/backend/internal/domain"
	"github.com/2132691342/NovelWorkbench/backend/internal/pkg"
	"github.com/2132691342/NovelWorkbench/backend/internal/repo"
)

// settingDefaultList 启动播种的默认配置（键白名单与其默认值、说明的唯一来源）。
var settingDefaultList = []domain.AppSettingDO{
	{SettingKey: "chat_history_count", SettingValue: "5", SettingDesc: "AI 对话历史注入条数（0-50）"},
	{SettingKey: "rag_top_k", SettingValue: "6", SettingDesc: "知识库检索默认召回条数（1-20）"},
	{SettingKey: "daily_word_goal", SettingValue: "2000", SettingDesc: "每日写作目标字数（100-100000）"},
}

// settingRangeMap 配置值合法区间（最小、最大）。
var settingRangeMap = map[string][2]int{
	"chat_history_count": {0, 50},
	"rag_top_k":          {1, 20},
	"daily_word_goal":    {100, 100000},
}

// SettingService 应用配置业务：默认值播种、查询与更新（值须为正整数）。
type SettingService struct {
	settingRepo *repo.SettingRepo // 应用配置仓储
}

// NewSettingService 构造应用配置服务。
func NewSettingService(settingRepo *repo.SettingRepo) *SettingService {
	return &SettingService{settingRepo: settingRepo}
}

// SeedDefaults 播种默认配置（幂等：按 setting_key 查缺补插）。
func (s *SettingService) SeedDefaults(ctx context.Context) error {
	for _, item := range settingDefaultList {
		existing, err := s.settingRepo.GetByKey(ctx, item.SettingKey)
		if err != nil {
			return err
		}
		if existing != nil {
			if existing.SettingDesc != item.SettingDesc {
				if err := s.settingRepo.UpdateDescription(ctx, item.SettingKey, item.SettingDesc); err != nil {
					return err
				}
			}
			continue
		}
		do := &domain.AppSettingDO{
			SettingULID:  pkg.NewULID(domain.IDPrefixSetting),
			SettingKey:   item.SettingKey,
			SettingValue: item.SettingValue,
			SettingDesc:  item.SettingDesc,
		}
		if err := s.settingRepo.Create(ctx, do); err != nil {
			return err
		}
	}
	return nil
}

// List 查询全部配置。
func (s *SettingService) List(ctx context.Context) ([]domain.SettingVO, error) {
	dos, err := s.settingRepo.List(ctx)
	if err != nil {
		return nil, err
	}
	vos := make([]domain.SettingVO, 0, len(dos))
	for i := range dos {
		vos = append(vos, domain.SettingVO{
			SettingKey:   dos[i].SettingKey,
			SettingValue: dos[i].SettingValue,
			SettingDesc:  dos[i].SettingDesc,
		})
	}
	return vos, nil
}

// Update 更新配置值（键须在白名单内，值须为正整数）。
func (s *SettingService) Update(ctx context.Context, req *domain.SettingUpdateREQ) (domain.SettingVO, error) {
	if !settingKeyAllowed(req.SettingKey) {
		return domain.SettingVO{}, pkg.NewBizError(domain.RespCodeInvalidParam, "不支持的配置键")
	}
	value, err := strconv.Atoi(req.SettingValue)
	if err != nil {
		return domain.SettingVO{}, pkg.NewBizError(domain.RespCodeInvalidParam, "配置值须为整数")
	}
	valueRange, ok := settingRangeMap[req.SettingKey]
	if !ok || value < valueRange[0] || value > valueRange[1] {
		return domain.SettingVO{}, pkg.NewBizError(domain.RespCodeInvalidParam, "配置值超出允许范围")
	}
	existing, err := s.settingRepo.GetByKey(ctx, req.SettingKey)
	if err != nil {
		return domain.SettingVO{}, err
	}
	if existing == nil {
		return domain.SettingVO{}, pkg.NewBizError(domain.RespCodeNotFound, "配置不存在")
	}
	if err := s.settingRepo.UpdateValue(ctx, req.SettingKey, req.SettingValue); err != nil {
		return domain.SettingVO{}, err
	}
	return domain.SettingVO{
		SettingKey:   req.SettingKey,
		SettingValue: req.SettingValue,
		SettingDesc:  existing.SettingDesc,
	}, nil
}

// settingKeyAllowed 判断配置键是否在白名单内。
func settingKeyAllowed(settingKey string) bool {
	for _, item := range settingDefaultList {
		if item.SettingKey == settingKey {
			return true
		}
	}
	return false
}
