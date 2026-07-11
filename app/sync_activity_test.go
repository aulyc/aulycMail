package app

import (
	"strings"
	"testing"

	"aulyc.local/aulycmail/internal/activitylog"
	syncengine "aulyc.local/aulycmail/internal/sync"
)

func TestSyncActivitySummaryUsesTerminalStatusAndCounts(t *testing.T) {
	t.Parallel()

	result := syncengine.MessageSyncResult{Added: 6, Removed: 2}
	for _, tt := range []struct {
		status string
		want   []string
	}{
		{status: activitylog.StatusSuccess, want: []string{"同步成功", "新增 6 封", "移除 2 封"}},
		{status: activitylog.StatusPartial, want: []string{"部分完成", "新增 6 封", "移除 2 封"}},
		{status: activitylog.StatusCancelled, want: []string{"同步已取消", "新增 6 封", "移除 2 封"}},
		{status: activitylog.StatusFailed, want: []string{"同步失败"}},
	} {
		tt := tt
		t.Run(tt.status, func(t *testing.T) {
			t.Parallel()
			got := syncActivitySummary(result, tt.status)
			for _, fragment := range tt.want {
				if !strings.Contains(got, fragment) {
					t.Fatalf("summary %q does not contain %q", got, fragment)
				}
			}
		})
	}
}
