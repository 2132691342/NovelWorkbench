// Package pkg 放置与业务无关的通用工具：ULID 生成、响应包络、业务错误、JWT。
package pkg

import (
	"crypto/rand"
	"strings"
	"time"

	"github.com/oklog/ulid/v2"
)

// NewULID 生成带业务前缀的 ULID 字符串，格式：PREFIX_26位ULID（大写）。
// prefix 必须来自 domain.IDPrefix* 常量（如 "PRJ_"），便于日志与数据排查。
func NewULID(prefix string) string {
	entropy := ulid.Monotonic(rand.Reader, 0)
	id := ulid.MustNew(ulid.Timestamp(time.Now()), entropy)
	return prefix + strings.ToUpper(id.String())
}

// IsULID 校验 value 是否形如 PREFIX_ULID 的合法业务主键；prefix 为空则只校验 ULID 段。
func IsULID(value string, prefix string) bool {
	idx := strings.LastIndex(value, "_")
	if idx <= 0 {
		return false // 无下划线分隔
	}
	if prefix != "" && value[:idx+1] != prefix {
		return false // 前缀不匹配
	}
	_, err := ulid.Parse(value[idx+1:])
	return err == nil
}
