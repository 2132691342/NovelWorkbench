package service

import (
	"testing"

	"github.com/2132691342/NovelWorkbench/backend/internal/domain"
)

// TestResolveUsageRange 覆盖区间解析：缺省近 14 天、非法日期回退、起止倒置自动纠正。
func TestResolveUsageRange(t *testing.T) {
	toDate, fromDate := resolveUsageRange("", "")
	if !isDateText(toDate) || !isDateText(fromDate) {
		t.Fatalf("默认区间应为合法日期: %s ~ %s", fromDate, toDate)
	}
	if fromDate >= toDate {
		t.Fatalf("默认区间起点应早于终点: %s ~ %s", fromDate, toDate)
	}
	toDate, fromDate = resolveUsageRange("2026-01-10", "2026-01-05")
	if fromDate != "2026-01-05" || toDate != "2026-01-10" {
		t.Fatalf("倒置区间应自动纠正，实际 %s ~ %s", fromDate, toDate)
	}
	toDate, fromDate = resolveUsageRange("bad-date", "2026-02-01")
	if !isDateText(fromDate) || !isDateText(toDate) {
		t.Fatalf("非法起点应回退合法日期，实际 %s ~ %s", fromDate, toDate)
	}
	if fromDate > toDate {
		t.Fatalf("回退后仍须保证起点不大于终点: %s ~ %s", fromDate, toDate)
	}
}

// TestFillUsageDaily 覆盖补零：缺失日期补空记录，已有数据保持原值。
func TestFillUsageDaily(t *testing.T) {
	rows := []domain.ModelUsageDailyVO{
		{StatDate: "2026-03-02", CallCount: 3, TotalTokens: 120},
	}
	filled := fillUsageDaily(rows, "2026-03-01", "2026-03-03")
	if len(filled) != 3 {
		t.Fatalf("len(filled) = %d, want 3", len(filled))
	}
	if filled[0].StatDate != "2026-03-01" || filled[0].CallCount != 0 {
		t.Fatalf("缺失日应为零值: %+v", filled[0])
	}
	if filled[1].CallCount != 3 || filled[1].TotalTokens != 120 {
		t.Fatalf("已有数据不应被改写: %+v", filled[1])
	}
	if filled[2].StatDate != "2026-03-03" {
		t.Fatalf("尾部日期缺失: %+v", filled[2])
	}
}
