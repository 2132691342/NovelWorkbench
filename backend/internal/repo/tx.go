package repo

import "gorm.io/gorm"

// WithTx 在事务中执行 fn；fn 返回错误时自动回滚。
func WithTx(db *gorm.DB, fn func(tx *gorm.DB) error) error {
	return db.Transaction(fn)
}
