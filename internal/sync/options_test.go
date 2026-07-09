package sync

import (
	"testing"
	"time"

	"github.com/aulyc/aulycmail/internal/account"
	"github.com/aulyc/aulycmail/internal/folder"
)

func TestShouldRunFullUIDSearch(t *testing.T) {
	now := time.Date(2026, 7, 8, 12, 0, 0, 0, time.UTC)
	lastWeek := now.Add(-7 * 24 * time.Hour)
	yesterday := now.Add(-24 * time.Hour)

	tests := []struct {
		name               string
		f                  *folder.Folder
		opts               MessageSyncOptions
		uidValidityChanged bool
		localCount         int
		want               bool
	}{
		{
			name: "uid validity change forces full",
			f:    &folder.Folder{LastFullSync: &yesterday},
			opts: MessageSyncOptions{
				Strategy:              account.SyncStrategyIncremental,
				FullCheckIntervalDays: 7,
			},
			uidValidityChanged: true,
			localCount:         20,
			want:               true,
		},
		{
			name: "full strategy always full",
			f:    &folder.Folder{LastFullSync: &yesterday},
			opts: MessageSyncOptions{
				Strategy: account.SyncStrategyFull,
			},
			localCount: 20,
			want:       true,
		},
		{
			name: "empty local folder full syncs first",
			f:    &folder.Folder{},
			opts: MessageSyncOptions{
				Strategy:              account.SyncStrategyIncremental,
				FullCheckIntervalDays: 7,
			},
			localCount: 0,
			want:       true,
		},
		{
			name: "manual full check skips scheduled full",
			f:    &folder.Folder{LastFullSync: &lastWeek},
			opts: MessageSyncOptions{
				Strategy:              account.SyncStrategyIncremental,
				FullCheckIntervalDays: 0,
			},
			localCount: 20,
			want:       false,
		},
		{
			name: "due interval runs full",
			f:    &folder.Folder{LastFullSync: &lastWeek},
			opts: MessageSyncOptions{
				Strategy:              account.SyncStrategyIncremental,
				FullCheckIntervalDays: 7,
			},
			localCount: 20,
			want:       true,
		},
		{
			name: "fresh interval uses incremental",
			f:    &folder.Folder{LastFullSync: &yesterday},
			opts: MessageSyncOptions{
				Strategy:              account.SyncStrategyIncremental,
				FullCheckIntervalDays: 7,
			},
			localCount: 20,
			want:       false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := shouldRunFullUIDSearch(tt.f, tt.opts, tt.uidValidityChanged, tt.localCount, now)
			if got != tt.want {
				t.Fatalf("shouldRunFullUIDSearch() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestBodyFetchOptionsFromAccount(t *testing.T) {
	tests := []struct {
		name string
		acc  *account.Account
		want BodyFetchOptions
	}{
		{
			name: "on demand disables background fetch",
			acc:  &account.Account{BodyDownloadPolicy: account.BodyDownloadOnDemand},
			want: BodyFetchOptions{Enabled: false},
		},
		{
			name: "recent uses configured days",
			acc:  &account.Account{BodyDownloadPolicy: account.BodyDownloadRecent, BodyDownloadDays: 365},
			want: BodyFetchOptions{Enabled: true, Days: 365},
		},
		{
			name: "all uses legacy all-days sentinel",
			acc:  &account.Account{BodyDownloadPolicy: account.BodyDownloadAll},
			want: BodyFetchOptions{Enabled: true, Days: 0},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := BodyFetchOptionsFromAccount(tt.acc)
			if got != tt.want {
				t.Fatalf("BodyFetchOptionsFromAccount() = %+v, want %+v", got, tt.want)
			}
		})
	}
}
