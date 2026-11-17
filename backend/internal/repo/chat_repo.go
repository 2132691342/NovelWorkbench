package repo

import (
	"context"
	"fmt"

	"gorm.io/gorm"

	"github.com/2132691342/NovelWorkbench/backend/internal/domain"
	"github.com/2132691342/NovelWorkbench/backend/internal/pkg"
)

// ChatRepo AI 会话与聊天消息持久化仓储（含 AI 字数统计与上下文装配专用查询）。
type ChatRepo struct {
	db *gorm.DB
}

// NewChatRepo 构造对话仓储。
func NewChatRepo(db *gorm.DB) *ChatRepo {
	return &ChatRepo{db: db}
}

// CreateSession 插入会话记录。
func (r *ChatRepo) CreateSession(ctx context.Context, do *domain.ChatSessionDO) error {
	if err := r.db.WithContext(ctx).Create(do).Error; err != nil {
		return fmt.Errorf("create chat session: %w", err)
	}
	return nil
}

// GetSession 按 ULID 查询会话，不存在时返回 (nil, nil)。
func (r *ChatRepo) GetSession(ctx context.Context, sessionULID string) (*domain.ChatSessionDO, error) {
	var do domain.ChatSessionDO
	err := r.db.WithContext(ctx).Where("session_ulid = ?", sessionULID).First(&do).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("get chat session: %w", err)
	}
	return &do, nil
}

// ListSessionsByProject 按项目查询会话（联消息聚合条数与最后活跃时间，按最后活跃倒序）。
func (r *ChatRepo) ListSessionsByProject(ctx context.Context, projectULID string) ([]domain.ChatSessionVO, error) {
	sql := `SELECT s.session_ulid, s.project_ulid, s.session_name,
			COUNT(m.message_ulid) AS message_count,
			FLOOR(EXTRACT(EPOCH FROM COALESCE(MAX(m.created_at), s.created_at)) * 1000)::bigint AS updated_at_ms
		FROM chat_sessions s
		LEFT JOIN chat_messages m ON m.session_ulid = s.session_ulid
		WHERE s.project_ulid = ?
		GROUP BY s.session_ulid
		ORDER BY updated_at_ms DESC`
	var vos []domain.ChatSessionVO
	if err := r.db.WithContext(ctx).Raw(sql, projectULID).Scan(&vos).Error; err != nil {
		return nil, fmt.Errorf("list chat sessions: %w", err)
	}
	return vos, nil
}

// UpdateSessionName 更新会话名（会话名缺省时由首条消息回填）。
func (r *ChatRepo) UpdateSessionName(ctx context.Context, sessionULID, sessionName string) error {
	result := r.db.WithContext(ctx).Model(&domain.ChatSessionDO{}).
		Where("session_ulid = ?", sessionULID).
		Update("session_name", sessionName)
	if result.Error != nil {
		return fmt.Errorf("update chat session name: %w", result.Error)
	}
	return nil
}

// DeleteSessionWithMessages 删除会话并级联删除其全部消息（同事务）。
func (r *ChatRepo) DeleteSessionWithMessages(ctx context.Context, sessionULID string) error {
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("session_ulid = ?", sessionULID).Delete(&domain.ChatMessageDO{}).Error; err != nil {
			return fmt.Errorf("delete chat messages: %w", err)
		}
		if err := tx.Where("session_ulid = ?", sessionULID).Delete(&domain.ChatSessionDO{}).Error; err != nil {
			return fmt.Errorf("delete chat session: %w", err)
		}
		return nil
	})
	return err
}

// CreateMessage 插入聊天消息。
func (r *ChatRepo) CreateMessage(ctx context.Context, do *domain.ChatMessageDO) error {
	if err := r.db.WithContext(ctx).Create(do).Error; err != nil {
		return fmt.Errorf("create chat message: %w", err)
	}
	return nil
}

// ListMessagesBySession 查询会话消息（created_at 升序，ULID 兜底保序）。
func (r *ChatRepo) ListMessagesBySession(ctx context.Context, sessionULID string) ([]domain.ChatMessageDO, error) {
	var dos []domain.ChatMessageDO
	err := r.db.WithContext(ctx).Where("session_ulid = ?", sessionULID).
		Order("created_at ASC, message_ulid ASC").Find(&dos).Error
	if err != nil {
		return nil, fmt.Errorf("list chat messages: %w", err)
	}
	return dos, nil
}

// UpsertAIWordDelta 累加某日 AI 生成字数（存在则累加 ai_word_delta，不存在则新建，与 UpsertWordDelta 同语义）。
func (r *ChatRepo) UpsertAIWordDelta(ctx context.Context, projectULID, statDate string, delta int64) error {
	db := r.db.WithContext(ctx)
	var stat domain.WordCountStatDO
	err := db.Where("project_ulid = ? AND stat_date = ?", projectULID, statDate).First(&stat).Error
	switch {
	case err == nil:
		if err := db.Model(&domain.WordCountStatDO{}).
			Where("word_stat_ulid = ?", stat.WordStatULID).
			Update("ai_word_delta", gorm.Expr("ai_word_delta + ?", delta)).Error; err != nil {
			return fmt.Errorf("update ai word stat: %w", err)
		}
	case err == gorm.ErrRecordNotFound:
		do := &domain.WordCountStatDO{
			WordStatULID: pkg.NewULID(domain.IDPrefixWordStat),
			ProjectULID:  projectULID,
			StatDate:     statDate,
			AIWordDelta:  delta,
		}
		if err := db.Create(do).Error; err != nil {
			return fmt.Errorf("create ai word stat: %w", err)
		}
	default:
		return fmt.Errorf("get ai word stat: %w", err)
	}
	return nil
}

// ListChapterMetaByProject 查询项目章节元信息（仅排序号/名称/梗概，供上下文装配定位前后章）。
func (r *ChatRepo) ListChapterMetaByProject(ctx context.Context, projectULID string) ([]domain.ChapterDO, error) {
	var dos []domain.ChapterDO
	err := r.db.WithContext(ctx).Model(&domain.ChapterDO{}).
		Select("chapter_ulid", "chapter_name", "chapter_order", "summary_text").
		Where("project_ulid = ?", projectULID).
		Order("chapter_order ASC").Find(&dos).Error
	if err != nil {
		return nil, fmt.Errorf("list chapter meta: %w", err)
	}
	return dos, nil
}

// ListLoreBriefsByProject 查询项目设定条目（仅名称/类型/内容；note 为私人笔记，SQL 层即排除）。
func (r *ChatRepo) ListLoreBriefsByProject(ctx context.Context, projectULID string) ([]domain.LoreDO, error) {
	var dos []domain.LoreDO
	err := r.db.WithContext(ctx).Model(&domain.LoreDO{}).
		Select("lore_ulid", "lore_name", "lore_type_code", "content_text").
		Where("project_ulid = ? AND lore_type_code != ?", projectULID, loreTypeCodeNote).
		Order("sort_order ASC, created_at ASC").Find(&dos).Error
	if err != nil {
		return nil, fmt.Errorf("list lore briefs: %w", err)
	}
	return dos, nil
}

// loreTypeCodeNote 创作笔记类型：作者私人笔记，禁止注入 AI 上下文。
const loreTypeCodeNote = "note"
