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
