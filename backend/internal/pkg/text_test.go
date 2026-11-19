// 本文件覆盖文本工具：中英混排字数统计与多字节安全的定长截断。

package pkg

import "testing"

// TestTruncateText 验证截断按字符而非字节进行，短文本原样返回。
func TestTruncateText(t *testing.T) {
	if got := TruncateText("小说工作台", 3); got != "小说工" {
		t.Fatalf("TruncateText 中文截断 = %q, want 小说工", got)
	}
	if got := TruncateText("novel", 10); got != "novel" {
		t.Fatalf("短文本不应被截断，实际 %q", got)
	}
	if got := TruncateText("novel", 0); got != "" {
		t.Fatalf("零长度应返回空串，实际 %q", got)
	}
	if got := TruncateText("雨夜追车", 2); len([]rune(got)) != 2 {
		t.Fatalf("截断后字符数 = %d, want 2", len([]rune(got)))
	}
}

// TestCountWords 验证 CJK 逐字计数、拉丁词按空白计数。
func TestCountWords(t *testing.T) {
	if got := CountWords("雨夜追车"); got != 4 {
		t.Fatalf("中文计数 = %d, want 4", got)
	}
	if got := CountWords("hello brave new world"); got != 4 {
		t.Fatalf("英文计数 = %d, want 4", got)
	}
	if got := CountWords("雨夜 chase"); got != 3 {
		t.Fatalf("中英混排计数 = %d, want 3", got)
	}
}
