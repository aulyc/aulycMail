// Package credentials provides secure credential storage with fallback support
package credentials

import (
	"database/sql"
	"fmt"

	"github.com/aulyc/aulycmail/internal/crypto"
	"github.com/aulyc/aulycmail/internal/logging"
	"github.com/rs/zerolog"
	gokeyring "github.com/zalando/go-keyring"
)

const serviceName = "aulycmail"

// Store provides credential storage with OS keyring and encrypted DB fallback
type Store struct {
	db             *sql.DB
	encryptor      *crypto.Encryptor
	keyringEnabled bool
	log            zerolog.Logger
}

// NewStore creates a new credential store
// It tries to use the OS keyring, falling back to encrypted database storage
func NewStore(db *sql.DB, dataDir string) (*Store, error) {
	log := logging.WithComponent("credentials")

	// Create encryptor for fallback storage
	encryptor, err := crypto.NewEncryptor(dataDir)
	if err != nil {
		return nil, fmt.Errorf("failed to create encryptor: %w", err)
	}

	// Test if keyring is available
	keyringEnabled := testKeyring()
	if keyringEnabled {
		log.Info().Msg("OS keyring available, using as primary credential storage")
	} else {
		log.Warn().Msg("OS keyring not available, using encrypted database storage")
	}

	return &Store{
		db:             db,
		encryptor:      encryptor,
		keyringEnabled: keyringEnabled,
		log:            log,
	}, nil
}

// testKeyring checks if the OS keyring is available and functional
func testKeyring() bool {
	testKey := "aulycmail-test-keyring-check"
	testValue := "test"

	// Try to set a test value
	err := gokeyring.Set(serviceName, testKey, testValue)
	if err != nil {
		return false
	}

	// Clean up test value
	_ = gokeyring.Delete(serviceName, testKey)

	return true
}

// SetPassword stores a password for an account
func (s *Store) SetPassword(accountID, password string) error {
	if password == "" {
		return nil
	}

	// Try OS keyring first if available
	if s.keyringEnabled {
		err := gokeyring.Set(serviceName, accountID, password)
		if err == nil {
			s.log.Debug().Str("account_id", accountID).Msg("Password stored in OS keyring")
			// Clear any fallback storage
			s.clearDBPassword(accountID)
			return nil
		}
		s.log.Warn().Err(err).Msg("Failed to store in OS keyring, using fallback")
	}

	// Fallback to encrypted database storage
	encrypted, err := s.encryptor.Encrypt(password)
	if err != nil {
		return fmt.Errorf("failed to encrypt password: %w", err)
	}

	_, err = s.db.Exec(
		"UPDATE accounts SET encrypted_password = ? WHERE id = ?",
		encrypted, accountID,
	)
	if err != nil {
		return fmt.Errorf("failed to store encrypted password: %w", err)
	}

	s.log.Debug().Str("account_id", accountID).Msg("Password stored in encrypted database")
	return nil
}

// GetPassword retrieves a password for an account
func (s *Store) GetPassword(accountID string) (string, error) {
	// Try OS keyring first if available
	if s.keyringEnabled {
		password, err := gokeyring.Get(serviceName, accountID)
		if err == nil {
			return password, nil
		}
		if err != gokeyring.ErrNotFound {
			s.log.Warn().Err(err).Msg("Error reading from OS keyring, trying fallback")
		}
	}

	// Try fallback encrypted database storage
	var encrypted sql.NullString
	err := s.db.QueryRow(
		"SELECT encrypted_password FROM accounts WHERE id = ?",
		accountID,
	).Scan(&encrypted)

	if err == sql.ErrNoRows {
		return "", ErrCredentialNotFound
	}
	if err != nil {
		return "", fmt.Errorf("failed to query password: %w", err)
	}

	if !encrypted.Valid || encrypted.String == "" {
		return "", ErrCredentialNotFound
	}

	// Decrypt
	password, err := s.encryptor.Decrypt(encrypted.String)
	if err != nil {
		return "", fmt.Errorf("failed to decrypt password: %w", err)
	}

	return password, nil
}

// DeletePassword removes a password for an account
func (s *Store) DeletePassword(accountID string) error {
	// Delete from OS keyring
	if s.keyringEnabled {
		_ = gokeyring.Delete(serviceName, accountID)
	}

	// Delete from database
	s.clearDBPassword(accountID)

	return nil
}

// clearDBPassword clears the encrypted password from the database
func (s *Store) clearDBPassword(accountID string) {
	_, _ = s.db.Exec("UPDATE accounts SET encrypted_password = NULL WHERE id = ?", accountID)
}

// DeleteAllCredentials removes all credentials for an account
func (s *Store) DeleteAllCredentials(accountID string) error {
	_ = s.DeletePassword(accountID)
	_ = s.DeleteSMTPPassword(accountID)
	return nil
}

// smtpPasswordKeyringKey returns the keyring slot used for the
// SMTP-specific password (only relevant when Account.SMTPUsername is set).
// Keeps the IMAP credential under accountID alone, so legacy entries are
// untouched by this addition.
func smtpPasswordKeyringKey(accountID string) string {
	return accountID + ":smtp"
}

// SetSMTPPassword stores the SMTP-specific password for an account. Used
// only when the account has a non-empty SMTPUsername (separate creds).
// Empty input is a no-op so the UI can submit a blank field on Update to
// mean "keep what's already there."
func (s *Store) SetSMTPPassword(accountID, password string) error {
	if password == "" {
		return nil
	}

	if s.keyringEnabled {
		err := gokeyring.Set(serviceName, smtpPasswordKeyringKey(accountID), password)
		if err == nil {
			s.log.Debug().Str("account_id", accountID).Msg("SMTP password stored in OS keyring")
			s.clearDBSMTPPassword(accountID)
			return nil
		}
		s.log.Warn().Err(err).Msg("Failed to store SMTP password in OS keyring, using fallback")
	}

	encrypted, err := s.encryptor.Encrypt(password)
	if err != nil {
		return fmt.Errorf("failed to encrypt SMTP password: %w", err)
	}
	if _, err := s.db.Exec(
		"UPDATE accounts SET encrypted_smtp_password = ? WHERE id = ?",
		encrypted, accountID,
	); err != nil {
		return fmt.Errorf("failed to store encrypted SMTP password: %w", err)
	}
	s.log.Debug().Str("account_id", accountID).Msg("SMTP password stored in encrypted database")
	return nil
}

// GetSMTPPassword retrieves the SMTP-specific password for an account.
// Mirrors GetPassword's keyring-first + DB-fallback shape, but uses the
// separate "<accountID>:smtp" keyring slot and the
// encrypted_smtp_password column.
func (s *Store) GetSMTPPassword(accountID string) (string, error) {
	if s.keyringEnabled {
		password, err := gokeyring.Get(serviceName, smtpPasswordKeyringKey(accountID))
		if err == nil {
			return password, nil
		}
		if err != gokeyring.ErrNotFound {
			s.log.Warn().Err(err).Msg("Error reading SMTP password from OS keyring, trying fallback")
		}
	}

	var encrypted sql.NullString
	err := s.db.QueryRow(
		"SELECT encrypted_smtp_password FROM accounts WHERE id = ?",
		accountID,
	).Scan(&encrypted)
	if err == sql.ErrNoRows {
		return "", ErrCredentialNotFound
	}
	if err != nil {
		return "", fmt.Errorf("failed to query SMTP password: %w", err)
	}
	if !encrypted.Valid || encrypted.String == "" {
		return "", ErrCredentialNotFound
	}
	password, err := s.encryptor.Decrypt(encrypted.String)
	if err != nil {
		return "", fmt.Errorf("failed to decrypt SMTP password: %w", err)
	}
	return password, nil
}

// DeleteSMTPPassword removes the SMTP-specific password for an account.
// Idempotent.
func (s *Store) DeleteSMTPPassword(accountID string) error {
	if s.keyringEnabled {
		_ = gokeyring.Delete(serviceName, smtpPasswordKeyringKey(accountID))
	}
	s.clearDBSMTPPassword(accountID)
	return nil
}

// clearDBSMTPPassword clears the encrypted SMTP password from the database.
func (s *Store) clearDBSMTPPassword(accountID string) {
	_, _ = s.db.Exec("UPDATE accounts SET encrypted_smtp_password = NULL WHERE id = ?", accountID)
}

// IsKeyringEnabled returns whether the OS keyring is being used
func (s *Store) IsKeyringEnabled() bool {
	return s.keyringEnabled
}
