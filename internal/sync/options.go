package sync

import (
	"time"

	"aulyc.local/aulycmail/internal/account"
	"aulyc.local/aulycmail/internal/folder"
)

// MessageSyncOptions separates local retention from the day-to-day IMAP sync
// strategy. RetentionDays controls local cleanup only; Strategy controls whether
// each cycle performs a full UID SEARCH or a fast incremental check.
type MessageSyncOptions struct {
	RetentionDays         int
	Strategy              string
	FullCheckIntervalDays int
}

// BodyFetchOptions controls the background body/attachment fetch after headers.
// Days follows the legacy body fetch convention: 0 means all messages.
type BodyFetchOptions struct {
	Enabled bool
	Days    int
}

func LegacyMessageSyncOptions(syncPeriodDays int) MessageSyncOptions {
	if syncPeriodDays < 0 {
		syncPeriodDays = 30
	}
	return MessageSyncOptions{
		RetentionDays:         syncPeriodDays,
		Strategy:              account.SyncStrategyFull,
		FullCheckIntervalDays: 0,
	}
}

func MessageSyncOptionsFromAccount(acc *account.Account) MessageSyncOptions {
	if acc == nil {
		return LegacyMessageSyncOptions(30)
	}

	retentionDays := acc.LocalRetentionDays
	if retentionDays < 0 {
		retentionDays = account.DefaultLocalRetentionDays
	}

	strategy := acc.SyncStrategy
	if strategy != account.SyncStrategyFull {
		strategy = account.SyncStrategyIncremental
	}

	fullCheckDays := acc.FullCheckIntervalDays
	if fullCheckDays < 0 {
		fullCheckDays = account.DefaultFullCheckIntervalDays
	}

	return MessageSyncOptions{
		RetentionDays:         retentionDays,
		Strategy:              strategy,
		FullCheckIntervalDays: fullCheckDays,
	}
}

func BodyFetchOptionsFromAccount(acc *account.Account) BodyFetchOptions {
	if acc == nil {
		return BodyFetchOptions{Enabled: true, Days: 30}
	}

	switch acc.BodyDownloadPolicy {
	case account.BodyDownloadOnDemand:
		return BodyFetchOptions{Enabled: false}
	case account.BodyDownloadAll:
		return BodyFetchOptions{Enabled: true, Days: 0}
	case account.BodyDownloadRecent:
		days := acc.BodyDownloadDays
		if days <= 0 {
			days = account.DefaultBodyDownloadRecentDays
		}
		return BodyFetchOptions{Enabled: true, Days: days}
	default:
		return BodyFetchOptions{Enabled: false}
	}
}

func shouldRunFullUIDSearch(f *folder.Folder, opts MessageSyncOptions, uidValidityChanged bool, localCount int, now time.Time) bool {
	if uidValidityChanged {
		return true
	}
	if opts.Strategy == account.SyncStrategyFull {
		return true
	}
	if localCount == 0 {
		return true
	}
	if opts.FullCheckIntervalDays == 0 {
		return false
	}
	if f.LastFullSync == nil {
		return true
	}
	return now.Sub(*f.LastFullSync) >= time.Duration(opts.FullCheckIntervalDays)*24*time.Hour
}
