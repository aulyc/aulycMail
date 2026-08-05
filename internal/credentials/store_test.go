package credentials

import (
	"database/sql"
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"aulyc.local/aulycmail/internal/account"
	"aulyc.local/aulycmail/internal/crypto"
	"aulyc.local/aulycmail/internal/database"
	"github.com/rs/zerolog"
)

func newCredentialTestStore(t *testing.T) (*Store, *database.DB, *account.Account) {
	t.Helper()
	db, err := database.Open(filepath.Join(t.TempDir(), "credentials.db"))
	if err != nil {
		t.Fatalf("database.Open: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := db.Migrate(); err != nil {
		t.Fatalf("Migrate: %v", err)
	}

	accountStore := account.NewStore(db)
	acc, err := accountStore.Create(&account.AccountConfig{
		Name:         "Credential Test",
		DisplayName:  "Credential Test",
		Email:        "credentials@example.com",
		IMAPHost:     "imap.example.com",
		IMAPPort:     993,
		IMAPSecurity: account.SecurityTLS,
		SMTPHost:     "smtp.example.com",
		SMTPPort:     587,
		SMTPSecurity: account.SecurityStartTLS,
		Username:     "credentials@example.com",
	})
	if err != nil {
		t.Fatalf("Create account: %v", err)
	}

	encryptor, err := crypto.NewEncryptor(t.TempDir())
	if err != nil {
		t.Fatalf("NewEncryptor: %v", err)
	}
	store := &Store{
		db:             db.DB,
		encryptor:      encryptor,
		keyringEnabled: false,
		log:            zerolog.Nop(),
	}
	return store, db, acc
}

func encryptedPasswordValue(t *testing.T, db *sql.DB, accountID string) sql.NullString {
	t.Helper()
	var encrypted sql.NullString
	if err := db.QueryRow("SELECT encrypted_password FROM accounts WHERE id = ?", accountID).Scan(&encrypted); err != nil {
		t.Fatalf("read encrypted_password: %v", err)
	}
	return encrypted
}

func encryptedSMTPPasswordValue(t *testing.T, db *sql.DB, accountID string) sql.NullString {
	t.Helper()
	var encrypted sql.NullString
	if err := db.QueryRow("SELECT encrypted_smtp_password FROM accounts WHERE id = ?", accountID).Scan(&encrypted); err != nil {
		t.Fatalf("read encrypted_smtp_password: %v", err)
	}
	return encrypted
}

func TestPasswordFallbackRoundTripAndDelete(t *testing.T) {
	store, db, acc := newCredentialTestStore(t)
	const password = "synthetic-imap-password"

	if err := store.SetPassword(acc.ID, password); err != nil {
		t.Fatalf("SetPassword: %v", err)
	}
	encrypted := encryptedPasswordValue(t, db.DB, acc.ID)
	if !encrypted.Valid || encrypted.String == "" || encrypted.String == password || strings.Contains(encrypted.String, password) {
		t.Fatalf("stored credential is not an opaque encrypted value: %#v", encrypted)
	}
	got, err := store.GetPassword(acc.ID)
	if err != nil || got != password {
		t.Fatalf("GetPassword = (%q, %v), want original password", got, err)
	}

	if err := store.SetPassword(acc.ID, ""); err != nil {
		t.Fatalf("empty SetPassword: %v", err)
	}
	got, err = store.GetPassword(acc.ID)
	if err != nil || got != password {
		t.Fatalf("empty SetPassword replaced existing value: (%q, %v)", got, err)
	}

	if err := store.DeletePassword(acc.ID); err != nil {
		t.Fatalf("DeletePassword: %v", err)
	}
	if _, err := store.GetPassword(acc.ID); !errors.Is(err, ErrCredentialNotFound) {
		t.Fatalf("GetPassword after delete error = %v, want ErrCredentialNotFound", err)
	}
	if encryptedPasswordValue(t, db.DB, acc.ID).Valid {
		t.Fatal("encrypted_password remains populated after delete")
	}
}

func TestSMTPPasswordFallbackRoundTripAndDeleteAll(t *testing.T) {
	store, db, acc := newCredentialTestStore(t)
	const imapPassword = "synthetic-imap-password"
	const smtpPassword = "synthetic-smtp-password"

	if err := store.SetPassword(acc.ID, imapPassword); err != nil {
		t.Fatalf("SetPassword: %v", err)
	}
	if err := store.SetSMTPPassword(acc.ID, smtpPassword); err != nil {
		t.Fatalf("SetSMTPPassword: %v", err)
	}
	encrypted := encryptedSMTPPasswordValue(t, db.DB, acc.ID)
	if !encrypted.Valid || encrypted.String == smtpPassword || strings.Contains(encrypted.String, smtpPassword) {
		t.Fatalf("stored SMTP credential is not encrypted: %#v", encrypted)
	}
	got, err := store.GetSMTPPassword(acc.ID)
	if err != nil || got != smtpPassword {
		t.Fatalf("GetSMTPPassword = (%q, %v), want original password", got, err)
	}

	if err := store.SetSMTPPassword(acc.ID, ""); err != nil {
		t.Fatalf("empty SetSMTPPassword: %v", err)
	}
	got, err = store.GetSMTPPassword(acc.ID)
	if err != nil || got != smtpPassword {
		t.Fatalf("empty SetSMTPPassword replaced existing value: (%q, %v)", got, err)
	}

	if err := store.DeleteAllCredentials(acc.ID); err != nil {
		t.Fatalf("DeleteAllCredentials: %v", err)
	}
	if _, err := store.GetPassword(acc.ID); !errors.Is(err, ErrCredentialNotFound) {
		t.Fatalf("IMAP credential after DeleteAllCredentials error = %v", err)
	}
	if _, err := store.GetSMTPPassword(acc.ID); !errors.Is(err, ErrCredentialNotFound) {
		t.Fatalf("SMTP credential after DeleteAllCredentials error = %v", err)
	}
}

func TestCredentialFallbackReportsMissingAndCorruptValues(t *testing.T) {
	store, db, acc := newCredentialTestStore(t)

	if _, err := store.GetPassword("missing-account"); !errors.Is(err, ErrCredentialNotFound) {
		t.Fatalf("missing account password error = %v, want ErrCredentialNotFound", err)
	}
	if _, err := store.GetSMTPPassword(acc.ID); !errors.Is(err, ErrCredentialNotFound) {
		t.Fatalf("unset SMTP password error = %v, want ErrCredentialNotFound", err)
	}

	if _, err := db.DB.Exec("UPDATE accounts SET encrypted_password = ? WHERE id = ?", "corrupt-ciphertext", acc.ID); err != nil {
		t.Fatalf("seed corrupt password: %v", err)
	}
	if _, err := store.GetPassword(acc.ID); err == nil || !strings.Contains(err.Error(), "failed to decrypt password") {
		t.Fatalf("corrupt password error = %v, want decryption failure", err)
	}

	if _, err := db.DB.Exec("UPDATE accounts SET encrypted_smtp_password = ? WHERE id = ?", "corrupt-ciphertext", acc.ID); err != nil {
		t.Fatalf("seed corrupt SMTP password: %v", err)
	}
	if _, err := store.GetSMTPPassword(acc.ID); err == nil || !strings.Contains(err.Error(), "failed to decrypt SMTP password") {
		t.Fatalf("corrupt SMTP password error = %v, want decryption failure", err)
	}
}
