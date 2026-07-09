package app

import (
	"errors"
	"fmt"
	"time"

	"github.com/aulyc/aulycmail/internal/account"
	"github.com/aulyc/aulycmail/internal/certificate"
	"github.com/aulyc/aulycmail/internal/imap"
	"github.com/aulyc/aulycmail/internal/logging"
)

// ============================================================================
// Account API - Exposed to frontend via Wails bindings
// ============================================================================

// GetAccounts returns all configured accounts
func (a *App) GetAccounts() ([]*account.Account, error) {
	return a.accountStore.List()
}

// GetAccount returns a single account by ID
func (a *App) GetAccount(id string) (*account.Account, error) {
	return a.accountStore.Get(id)
}

// AddAccount creates a new email account
func (a *App) AddAccount(config account.AccountConfig) (*account.Account, error) {
	log := logging.WithComponent("app")

	// Create account in database
	acc, err := a.accountStore.Create(&config)
	if err != nil {
		log.Error().Err(err).Str("email", config.Email).Msg("Failed to create account")
		return nil, err
	}

	// Store password in credential store
	if config.Password != "" {
		if err := a.credStore.SetPassword(acc.ID, config.Password); err != nil {
			log.Error().Err(err).Str("account_id", acc.ID).Msg("Failed to store password")
			// Delete the account since we can't store credentials
			if delErr := a.accountStore.Delete(acc.ID); delErr != nil {
				log.Warn().Err(delErr).Str("account_id", acc.ID).Msg("Failed to roll back account after password storage failure")
			}
			return nil, fmt.Errorf("failed to store password: %w", err)
		}
	}

	// Store SMTP-specific password when the account uses separate SMTP
	// credentials (signalled by a non-empty SMTPUsername). Empty
	// SMTPPassword is silently ignored — "Same as incoming server" keeps
	// the SMTP keyring slot empty so the send path falls through to the
	// IMAP password.
	if config.SMTPUsername != "" && config.SMTPPassword != "" {
		if err := a.credStore.SetSMTPPassword(acc.ID, config.SMTPPassword); err != nil {
			log.Error().Err(err).Str("account_id", acc.ID).Msg("Failed to store SMTP password")
			if delErr := a.accountStore.Delete(acc.ID); delErr != nil {
				log.Warn().Err(delErr).Str("account_id", acc.ID).Msg("Failed to roll back account after SMTP password storage failure")
			}
			return nil, fmt.Errorf("failed to store SMTP password: %w", err)
		}
	}

	// Scale database connection pool for new account
	a.updateDBConnectionPool()

	// Start IDLE for the new account
	if a.idleManager != nil && acc.Enabled {
		a.idleManager.StartAccount(acc.ID, acc.Name)
	}

	log.Info().Str("account_id", acc.ID).Str("email", acc.Email).Msg("Account created")
	return acc, nil
}

// UpdateAccount updates an existing account
func (a *App) UpdateAccount(id string, config account.AccountConfig) (*account.Account, error) {
	log := logging.WithComponent("app")

	// Get existing account to check for sync period changes
	existingAcc, err := a.accountStore.Get(id)
	if err != nil {
		log.Error().Err(err).Str("account_id", id).Msg("Failed to get existing account")
		return nil, fmt.Errorf("failed to get existing account: %w", err)
	}
	if existingAcc == nil {
		return nil, fmt.Errorf("account not found: %s", id)
	}

	// Validate folder mappings if any are set
	folderPaths := map[string]string{
		"sent":    config.SentFolderPath,
		"drafts":  config.DraftsFolderPath,
		"trash":   config.TrashFolderPath,
		"spam":    config.SpamFolderPath,
		"archive": config.ArchiveFolderPath,
		"all":     config.AllMailFolderPath,
		"starred": config.StarredFolderPath,
	}

	for folderType, path := range folderPaths {
		if path != "" {
			f, err := a.folderStore.GetByPath(id, path)
			if err != nil {
				return nil, fmt.Errorf("error checking %s folder: %w", folderType, err)
			}
			if f == nil {
				return nil, fmt.Errorf("%s folder not found: %s", folderType, path)
			}
		}
	}

	// Check if any sync behavior changed
	syncSettingsChanged := existingAcc.SyncPeriodDays != config.SyncPeriodDays ||
		existingAcc.LocalRetentionDays != config.LocalRetentionDays ||
		existingAcc.SyncStrategy != config.SyncStrategy ||
		existingAcc.FullCheckIntervalDays != config.FullCheckIntervalDays ||
		existingAcc.BodyDownloadPolicy != config.BodyDownloadPolicy ||
		existingAcc.BodyDownloadDays != config.BodyDownloadDays

	acc, err := a.accountStore.Update(id, &config)
	if err != nil {
		log.Error().Err(err).Str("account_id", id).Msg("Failed to update account")
		return nil, err
	}

	// Update password in credential store if provided
	if config.Password != "" {
		if err := a.credStore.SetPassword(id, config.Password); err != nil {
			log.Error().Err(err).Str("account_id", id).Msg("Failed to update password")
			return nil, fmt.Errorf("failed to update password: %w", err)
		}
	}

	// Update SMTP-specific password. SMTPUsername=="" means "Same as
	// incoming server" — drop any previously stored SMTP password so the
	// send path falls through to the IMAP credential. SMTPUsername!=""
	// with a non-empty SMTPPassword writes the new value; blank password
	// on edit means "keep the existing one."
	if config.SMTPUsername == "" {
		_ = a.credStore.DeleteSMTPPassword(id)
	}
	if config.SMTPUsername != "" && config.SMTPPassword != "" {
		if err := a.credStore.SetSMTPPassword(id, config.SMTPPassword); err != nil {
			log.Error().Err(err).Str("account_id", id).Msg("Failed to update SMTP password")
			return nil, fmt.Errorf("failed to update SMTP password: %w", err)
		}
	}

	// If sync settings changed, cancel any running sync and trigger a new one
	if syncSettingsChanged && a.syncScheduler != nil {
		log.Info().
			Str("account_id", id).
			Int("old_local_retention_days", existingAcc.LocalRetentionDays).
			Int("new_local_retention_days", config.LocalRetentionDays).
			Str("old_sync_strategy", existingAcc.SyncStrategy).
			Str("new_sync_strategy", config.SyncStrategy).
			Msg("Sync settings changed, cancelling current sync and triggering new sync")

		a.syncScheduler.CancelSync(id)
		// Small delay to allow cancellation to complete
		go func() {
			defer recoverPanic("app", "sync after account update")
			// time.Sleep(500 * time.Millisecond)
			a.syncScheduler.TriggerSync(id)
		}()
	}

	log.Info().Str("account_id", id).Msg("Account updated")
	return acc, nil
}

// RemoveAccount deletes an account and all its data
func (a *App) RemoveAccount(id string) error {
	log := logging.WithComponent("app")

	// Cascade: delete any shared mailboxes linked to this account
	legacyChildren, _ := a.accountStore.ListBySharedMailboxParent(id)
	for _, sm := range legacyChildren {
		log.Info().Str("sharedID", sm.ID).Str("parentID", id).Msg("Cascade deleting shared mailbox")
		if err := a.RemoveAccount(sm.ID); err != nil {
			log.Warn().Err(err).Str("sharedID", sm.ID).Msg("Failed to cascade-delete shared mailbox")
		}
	}

	// Stop IDLE for this account
	if a.idleManager != nil {
		a.idleManager.StopAccount(id)
	}

	// Close any IMAP connections for this account
	a.imapPool.CloseAccount(id)

	// Delete from database (cascades to folders, messages, etc.)
	if err := a.accountStore.Delete(id); err != nil {
		log.Error().Err(err).Str("account_id", id).Msg("Failed to delete account")
		return err
	}

	// Delete credentials from credential store
	if err := a.credStore.DeleteAllCredentials(id); err != nil {
		log.Warn().Err(err).Str("account_id", id).Msg("Failed to delete credentials")
	}

	// Scale database connection pool after removing account
	a.updateDBConnectionPool()

	log.Info().Str("account_id", id).Msg("Account removed")
	return nil
}

// ReorderAccounts updates the order of accounts
func (a *App) ReorderAccounts(ids []string) error {
	return a.accountStore.Reorder(ids)
}

// AccountIdentityGroup groups an account with its identities for the cross-account From dropdown
type AccountIdentityGroup struct {
	Account    *account.Account    `json:"account"`
	Identities []*account.Identity `json:"identities"`
}

// GetAllAccountIdentities returns all accounts with their identities in one call.
// Used by the inline composer to populate the cross-account From dropdown.
func (a *App) GetAllAccountIdentities() ([]AccountIdentityGroup, error) {
	accounts, err := a.accountStore.List()
	if err != nil {
		return nil, err
	}
	var groups []AccountIdentityGroup
	for _, acc := range accounts {
		if !acc.Enabled {
			continue
		}
		identities, err := a.accountStore.GetIdentities(acc.ID)
		if err != nil {
			return nil, err
		}
		groups = append(groups, AccountIdentityGroup{
			Account:    acc,
			Identities: identities,
		})
	}
	return groups, nil
}

// GetIdentities returns all identities for an account
func (a *App) GetIdentities(accountID string) ([]*account.Identity, error) {
	return a.accountStore.GetIdentities(accountID)
}

// CreateIdentity creates a new email identity for an account
func (a *App) CreateIdentity(accountID string, config account.IdentityConfig) (*account.Identity, error) {
	return a.accountStore.CreateIdentity(accountID, &config)
}

// UpdateIdentity updates an existing identity
func (a *App) UpdateIdentity(identityID string, config account.IdentityConfig) (*account.Identity, error) {
	return a.accountStore.UpdateIdentity(identityID, &config)
}

// DeleteIdentity deletes an identity (cannot delete the default identity)
func (a *App) DeleteIdentity(identityID string) error {
	return a.accountStore.DeleteIdentity(identityID)
}

// SetDefaultIdentity sets an identity as the default for sending
func (a *App) SetDefaultIdentity(accountID, identityID string) error {
	return a.accountStore.SetDefaultIdentity(accountID, identityID)
}

// ============================================================================
// Connection Testing
// ============================================================================

// ConnectionTestResult holds the result of a connection test
type ConnectionTestResult struct {
	Success             bool                         `json:"success"`
	Error               string                       `json:"error,omitempty"`
	CertificateRequired bool                         `json:"certificateRequired"`
	Certificate         *certificate.CertificateInfo `json:"certificate,omitempty"`
}

// TestConnection tests the IMAP/SMTP connection for an account config.
func (a *App) TestConnection(config account.AccountConfig) ConnectionTestResult {
	log := logging.WithComponent("app")

	// Validate config first
	if err := config.Validate(); err != nil {
		return ConnectionTestResult{Error: err.Error()}
	}

	// Create a temporary IMAP client to test connection
	clientConfig := imap.DefaultConfig()
	clientConfig.Host = config.IMAPHost
	clientConfig.Port = config.IMAPPort
	clientConfig.Security = imap.SecurityType(config.IMAPSecurity)
	clientConfig.Username = config.Username
	clientConfig.Password = config.Password
	clientConfig.AuthType = imap.AuthTypePassword
	clientConfig.TLSConfig = certificate.BuildTLSConfig(config.IMAPHost, a.certStore)

	client := imap.NewClient(clientConfig)

	if err := client.Connect(); err != nil {
		var certErr *certificate.Error
		if errors.As(err, &certErr) {
			return ConnectionTestResult{
				CertificateRequired: true,
				Certificate:         certErr.Info,
			}
		}
		log.Error().Err(err).Msg("Connection test failed")
		return ConnectionTestResult{Error: fmt.Sprintf("failed to connect: %v", err)}
	}
	defer client.Close()

	if err := client.Login(); err != nil {
		log.Error().Err(err).Msg("Login test failed")
		return ConnectionTestResult{Error: fmt.Sprintf("failed to login: %v", err)}
	}

	log.Info().Str("host", config.IMAPHost).Msg("Connection test successful")
	return ConnectionTestResult{Success: true}
}

// TestAccountConnection tests connectivity for an EXISTING account by reusing
// its stored credentials (the same config the sync pool uses). On success it
// stamps the account's last-successful-connection time. Used by the per-account
// "test connection" button in Settings → Accounts.
func (a *App) TestAccountConnection(accountID string) ConnectionTestResult {
	log := logging.WithComponent("app")

	cfg, err := a.getIMAPCredentials(accountID)
	if err != nil {
		return ConnectionTestResult{Error: fmt.Sprintf("failed to load account credentials: %v", err)}
	}

	client := imap.NewClient(*cfg)
	if err := client.Connect(); err != nil {
		var certErr *certificate.Error
		if errors.As(err, &certErr) {
			return ConnectionTestResult{CertificateRequired: true, Certificate: certErr.Info}
		}
		log.Error().Err(err).Str("account", accountID).Msg("Account connection test failed")
		return ConnectionTestResult{Error: fmt.Sprintf("failed to connect: %v", err)}
	}
	defer client.Close()

	if err := client.Login(); err != nil {
		log.Error().Err(err).Str("account", accountID).Msg("Account login test failed")
		return ConnectionTestResult{Error: fmt.Sprintf("failed to login: %v", err)}
	}

	a.recordAccountConnOK(accountID)
	log.Info().Str("account", accountID).Msg("Account connection test successful")
	return ConnectionTestResult{Success: true}
}

// recordAccountConnOK stamps the account's most-recent successful connection
// (test, send, or receive) as an RFC3339 timestamp in settings.
func (a *App) recordAccountConnOK(accountID string) {
	if a.settingsStore == nil || accountID == "" {
		return
	}
	_ = a.settingsStore.Set("conn_ok_"+accountID, time.Now().UTC().Format(time.RFC3339))
}

// GetAccountConnOK returns the RFC3339 timestamp of the account's last
// successful connection, or "" when there's none yet. The frontend renders it
// as a relative "X ago" hint next to the test-connection button.
func (a *App) GetAccountConnOK(accountID string) (string, error) {
	if a.settingsStore == nil {
		return "", nil
	}
	v, err := a.settingsStore.Get("conn_ok_" + accountID)
	if err != nil {
		return "", nil
	}
	return v, nil
}
