// 本文件实现审计字段自动填充：从请求上下文取当前登录用户，写入 created_by / updated_by。

package pkg

import (
	"context"

	"gorm.io/gorm"
)

// actorContextKey 请求上下文中当前操作人的私有键（避免与 gin 的字符串键混淆）。
type actorContextKey struct{}

// WithActor 把当前登录用户 ULID 放进 context，供审计回调读取。
func WithActor(ctx context.Context, userULID string) context.Context {
	if userULID == "" {
		return ctx
	}
	return context.WithValue(ctx, actorContextKey{}, userULID)
}

// ActorFromContext 读取当前操作人 ULID（未登录与系统任务返回空串）。
func ActorFromContext(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	actorULID, _ := ctx.Value(actorContextKey{}).(string)
	return actorULID
}

// RegisterActorCallbacks 注册审计字段自动填充回调（插入写创建人，更新写更新人）。
// 回调挂在 GORM 内置写入之前，因此结构体与 map 两种目标写法都能生效。
func RegisterActorCallbacks(db *gorm.DB) error {
	if err := db.Callback().Create().Before("gorm:create").
		Register("nw:fill_created_by", func(tx *gorm.DB) { fillActorColumn(tx, "CreatedBy", "created_by") }); err != nil {
		return err
	}
	return db.Callback().Update().Before("gorm:update").
		Register("nw:fill_updated_by", func(tx *gorm.DB) { fillActorColumn(tx, "UpdatedBy", "updated_by") })
}

// fillActorColumn 把当前操作人写入指定列：map 目标写列名键，结构体目标写属性名。
func fillActorColumn(db *gorm.DB, fieldName string, columnName string) {
	if db.Statement == nil || db.Statement.Schema == nil {
		return
	}
	actorULID := ActorFromContext(db.Statement.Context)
	if actorULID == "" || db.Statement.Schema.LookUpField(fieldName) == nil {
		return
	}
	switch dest := db.Statement.Dest.(type) {
	case map[string]any:
		if _, exists := dest[columnName]; !exists {
			dest[columnName] = actorULID
		}
	case []map[string]any:
		for _, destMap := range dest {
			if _, exists := destMap[columnName]; !exists {
				destMap[columnName] = actorULID
			}
		}
	default:
		db.Statement.SetColumn(fieldName, actorULID)
	}
}

// requestIDContextKey 请求链路 ID 的私有上下文键（与 actor 同理，避免字符串键冲突）。
type requestIDContextKey struct{}

// WithRequestID 把链路 ID 放进 context，供后台任务（工作流等）复用同一链路。
func WithRequestID(ctx context.Context, requestID string) context.Context {
	if requestID == "" {
		return ctx
	}
	return context.WithValue(ctx, requestIDContextKey{}, requestID)
}

// RequestIDFromContext 读取当前请求的链路 ID（非 HTTP 场景返回空串）。
func RequestIDFromContext(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	requestID, _ := ctx.Value(requestIDContextKey{}).(string)
	return requestID
}
