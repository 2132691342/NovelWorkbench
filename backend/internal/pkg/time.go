// 本文件实现时间格式转换：领域层时间一律以 RFC3339 字符串出参，避免毫秒整数歧义。

package pkg

import "time"

// FormatTime 把时间转成 RFC3339 字符串（零值返回空串，前端据此判空）。
func FormatTime(value time.Time) string {
	if value.IsZero() {
		return ""
	}
	return value.Format(time.RFC3339)
}
