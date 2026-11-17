package infra

import (
	"fmt"
	"strings"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"gorm.io/gorm/schema"
)

// ULIDNamingStrategy 修正 GORM 默认命名：XxxULID 会被推导为 xxx_ul_id，
// 与手写 SQL/JSON 约定的 xxx_ulid 不一致，这里统一归一为 xxx_ulid。
type ULIDNamingStrategy struct {
	schema.NamingStrategy // 内嵌默认策略，仅覆盖列名归一
}

// ColumnName 返回字段对应的列名，将默认推导的 _ul_id 后缀归一为 _ulid。
func (n ULIDNamingStrategy) ColumnName(table string, column string) string {
	name := n.NamingStrategy.ColumnName(table, column)
	if strings.HasSuffix(name, "_ul_id") {
		return strings.TrimSuffix(name, "_ul_id") + "_ulid"
	}
	return name
}

// NewPostgres 建立 Postgres 连接并配置连接池；连接或 Ping 失败返回错误，由调用方决定降级策略。
func NewPostgres(cfg *PostgresConfig) (*gorm.DB, error) {
	dsn := fmt.Sprintf(
		"host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		cfg.Host, cfg.Port, cfg.User, cfg.Password, cfg.DBName, cfg.SSLMode,
	)
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{
		Logger:         logger.Default.LogMode(logger.Warn),
		NamingStrategy: ULIDNamingStrategy{},
	})
	if err != nil {
		return nil, fmt.Errorf("open postgres: %w", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("get postgres pool: %w", err)
	}
	sqlDB.SetMaxOpenConns(cfg.MaxOpenConns)
	sqlDB.SetMaxIdleConns(cfg.MaxIdleConns)
	sqlDB.SetConnMaxLifetime(time.Duration(cfg.ConnMaxLifetimeMin) * time.Minute)
	if err := sqlDB.Ping(); err != nil {
		return nil, fmt.Errorf("ping postgres: %w", err)
	}
	return db, nil
}
