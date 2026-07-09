package database

import (
	"database/sql"
	"path/filepath"
	"strings"
	"testing"
)

func openTestDB(t *testing.T) *DB {
	t.Helper()
	path := filepath.Join(t.TempDir(), "test.db")
	db, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Migrate(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

func openTestDBAtMigration(t *testing.T, version int) *DB {
	t.Helper()
	path := filepath.Join(t.TempDir(), "test.db")
	db, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS migrations (
			version INTEGER PRIMARY KEY,
			applied_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)
	`); err != nil {
		_ = db.Close()
		t.Fatalf("create migrations table: %v", err)
	}
	for _, m := range migrations {
		if m.Version > version {
			break
		}
		if err := db.applyMigration(m); err != nil {
			_ = db.Close()
			t.Fatalf("apply migration %d: %v", m.Version, err)
		}
	}
	t.Cleanup(func() { db.Close() })
	return db
}

func applyMigrationsAfter(t *testing.T, db *DB, version int) {
	t.Helper()
	for _, m := range migrations {
		if m.Version <= version {
			continue
		}
		if err := db.applyMigration(m); err != nil {
			t.Fatalf("apply migration %d: %v", m.Version, err)
		}
	}
}

func applyMigrationVersion(t *testing.T, db *DB, version int) {
	t.Helper()
	for _, m := range migrations {
		if m.Version == version {
			if err := db.applyMigration(m); err != nil {
				t.Fatalf("apply migration %d: %v", m.Version, err)
			}
			return
		}
	}
	t.Fatalf("migration %d not found", version)
}

func tableExists(t *testing.T, db *DB, name string) bool {
	t.Helper()
	var count int
	if err := db.QueryRow(`
		SELECT COUNT(*)
		FROM sqlite_master
		WHERE type = 'table' AND name = ?
	`, name).Scan(&count); err != nil {
		t.Fatalf("check table %s: %v", name, err)
	}
	return count > 0
}

func columnExists(t *testing.T, db *DB, table, column string) bool {
	t.Helper()
	rows, err := db.Query(`PRAGMA table_info(` + table + `)`)
	if err != nil {
		t.Fatalf("read table info for %s: %v", table, err)
	}
	defer rows.Close()
	for rows.Next() {
		var cid, notNull, pk int
		var name, typ string
		var defaultValue sql.NullString
		if err := rows.Scan(&cid, &name, &typ, &notNull, &defaultValue, &pk); err != nil {
			t.Fatalf("scan table info for %s: %v", table, err)
		}
		if name == column {
			return true
		}
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate table info for %s: %v", table, err)
	}
	return false
}

func TestOpen(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.db")
	db, err := Open(path)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	t.Cleanup(func() { db.Close() })

	if db == nil {
		t.Fatal("Open() returned nil DB")
	}
}

func TestMigrate(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.db")
	db, err := Open(path)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	t.Cleanup(func() { db.Close() })

	if err := db.Migrate(); err != nil {
		t.Fatalf("Migrate() error = %v", err)
	}
}

func TestMigrateIdempotent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.db")
	db, err := Open(path)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	t.Cleanup(func() { db.Close() })

	if err := db.Migrate(); err != nil {
		t.Fatalf("first Migrate() error = %v", err)
	}
	if err := db.Migrate(); err != nil {
		t.Fatalf("second Migrate() error = %v", err)
	}
}

func TestUpdateIdleConns(t *testing.T) {
	db := openTestDB(t)

	tests := []struct {
		name        string
		numAccounts int
	}{
		{"zero accounts", 0},
		{"three accounts", 3},
		{"ten accounts", 10},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Verify no panic
			db.UpdateIdleConns(tt.numAccounts)
		})
	}
}

func TestCheckpoint(t *testing.T) {
	db := openTestDB(t)

	if err := db.Checkpoint(); err != nil {
		t.Fatalf("Checkpoint() error = %v", err)
	}
}

// TestMigrationV29_OAuthCompositeKey verifies the legacy OAuth compatibility
// migration: oauth_tokens uses composite PK (account_id, client_config_id) so
// old databases with multiple token rows still migrate deterministically.
func TestMigrationV29_OAuthCompositeKey(t *testing.T) {
	db := openTestDBAtMigration(t, 29)

	// Insert a test account row (oauth_tokens.account_id FK to accounts.id).
	// Schema defaults handle most columns; only NOT NULL non-default fields are explicit.
	if _, err := db.Exec(`
		INSERT INTO accounts (id, name, email, imap_host, smtp_host, username)
		VALUES ('acct-1', 'Test', 'user@example.com', 'imap.example.com', 'smtp.example.com', 'user@example.com')
	`); err != nil {
		t.Fatalf("insert account: %v", err)
	}

	// Insert mail-config token row
	if _, err := db.Exec(`
		INSERT INTO oauth_tokens (account_id, client_config_id, provider, expires_at, scopes)
		VALUES ('acct-1', 'google-mail', 'google', CURRENT_TIMESTAMP, '[]')
	`); err != nil {
		t.Fatalf("insert mail token row: %v", err)
	}

	// Insert extension-config token row for same account — should succeed
	if _, err := db.Exec(`
		INSERT INTO oauth_tokens (account_id, client_config_id, provider, expires_at, scopes)
		VALUES ('acct-1', 'google-extensions', 'google', CURRENT_TIMESTAMP, '[]')
	`); err != nil {
		t.Fatalf("insert extension token row failed (composite PK should allow it): %v", err)
	}

	// Verify both rows exist
	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM oauth_tokens WHERE account_id = 'acct-1'`).Scan(&count); err != nil {
		t.Fatalf("count rows: %v", err)
	}
	if count != 2 {
		t.Fatalf("expected 2 token rows for account, got %d", count)
	}

	// Duplicate (account_id, client_config_id) must violate the composite PK
	if _, err := db.Exec(`
		INSERT INTO oauth_tokens (account_id, client_config_id, provider, expires_at, scopes)
		VALUES ('acct-1', 'google-mail', 'google', CURRENT_TIMESTAMP, '[]')
	`); err == nil {
		t.Fatal("expected composite PK conflict on duplicate (account_id, client_config_id), got no error")
	}
}

func TestMigrationV42_TrustedCertificatesScopedByHost(t *testing.T) {
	db := openTestDB(t)

	if _, err := db.Exec(`
		INSERT INTO trusted_certificates (id, fingerprint, host, subject, issuer)
		VALUES
			('cert-1', 'fp-shared', 'imap.example.com', 'Subject', 'Issuer'),
			('cert-2', 'fp-shared', 'smtp.example.com', 'Subject', 'Issuer')
	`); err != nil {
		t.Fatalf("same fingerprint should be allowed for different hosts: %v", err)
	}

	if _, err := db.Exec(`
		INSERT INTO trusted_certificates (id, fingerprint, host, subject, issuer)
		VALUES ('cert-3', 'fp-shared', 'imap.example.com', 'Subject', 'Issuer')
	`); err == nil {
		t.Fatal("expected duplicate host+fingerprint to be rejected")
	}
}

func TestMigrationV46_FTSUpdateTriggerIgnoresFlagOnlyUpdates(t *testing.T) {
	db := openTestDB(t)

	var sql string
	if err := db.QueryRow(`
		SELECT sql
		FROM sqlite_master
		WHERE type = 'trigger' AND name = 'messages_fts_update'
	`).Scan(&sql); err != nil {
		t.Fatalf("read messages_fts_update trigger: %v", err)
	}
	if !strings.Contains(sql, "AFTER UPDATE OF subject, from_name, from_email, to_list, cc_list, snippet, body_text ON messages") {
		t.Fatalf("messages_fts_update should be column-scoped, got:\n%s", sql)
	}
	if strings.Contains(sql, "is_read") || strings.Contains(sql, "is_starred") || strings.Contains(sql, "is_deleted") {
		t.Fatalf("messages_fts_update should not reference flag columns, got:\n%s", sql)
	}
}

func TestMigrationV47_DropsLegacyRemoteContactSchema(t *testing.T) {
	db := openTestDBAtMigration(t, 46)

	if _, err := db.Exec(`
		INSERT INTO accounts (id, name, email, imap_host, smtp_host, username, encrypted_access_token, encrypted_refresh_token)
		VALUES ('acct-1', 'Test', 'user@example.com', 'imap.example.com', 'smtp.example.com', 'user@example.com', 'old-access', 'old-refresh');

		INSERT INTO contact_records (id, source, kind, fn, created_at, updated_at)
		VALUES ('local-rec', 'local', 'manual', 'Local', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP);
		INSERT INTO contact_emails (record_id, email, is_primary)
		VALUES ('local-rec', 'local@example.com', 1);

		INSERT INTO contact_sources (id, name, type, url, username, enabled, sync_interval)
		VALUES ('src-1', 'Remote', 'carddav', 'https://example.com/dav', 'user', 1, 60);
		INSERT INTO contact_source_addressbooks (id, source_id, path, name, enabled)
		VALUES ('ab-1', 'src-1', '/contacts/', 'Contacts', 1);
		INSERT INTO contact_source_oauth (source_id, provider, expires_at, scopes, client_config_id)
		VALUES ('src-1', 'google', CURRENT_TIMESTAMP, '[]', 'google-extensions');
		INSERT INTO contact_records (id, source, source_ref, fn, vcard_raw, created_at, updated_at)
		VALUES ('remote-rec', 'carddav', 'ab-1', 'Remote', 'BEGIN:VCARD', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP);
		INSERT INTO contact_emails (record_id, email, is_primary)
		VALUES ('remote-rec', 'remote@example.com', 1);
		INSERT INTO carddav_record_state (record_id, addressbook_id, href, etag)
		VALUES ('remote-rec', 'ab-1', '/contacts/remote.vcf', 'etag');

		INSERT INTO oauth_tokens (account_id, client_config_id, provider, expires_at, scopes, encrypted_access_token, encrypted_refresh_token)
		VALUES ('acct-1', 'google-mail', 'google', CURRENT_TIMESTAMP, '[]', 'token-access', 'token-refresh');
		INSERT INTO extension_secrets (extension, key, encrypted_value, created_at)
		VALUES ('contacts', 'token', 'secret', 1);
		CREATE TABLE user_oauth_clients (id TEXT PRIMARY KEY);
		CREATE TABLE user_oauth_slot_aliases (id TEXT PRIMARY KEY);
		INSERT INTO user_oauth_clients (id) VALUES ('client-1');
		INSERT INTO user_oauth_slot_aliases (id) VALUES ('alias-1');
		INSERT INTO settings (key, value)
		VALUES ('oauth_active_choice:google-mail', 'custom');
	`); err != nil {
		t.Fatalf("seed legacy schema: %v", err)
	}

	if err := db.Migrate(); err != nil {
		t.Fatalf("migrate to v47: %v", err)
	}

	for _, table := range []string{
		"carddav_record_state",
		"contact_source_oauth",
		"contact_source_addressbooks",
		"contact_sources",
		"oauth_tokens",
		"extension_secrets",
		"user_oauth_clients",
		"user_oauth_slot_aliases",
	} {
		if tableExists(t, db, table) {
			t.Fatalf("legacy table %s should be dropped", table)
		}
	}

	for _, col := range []string{"encrypted_access_token", "encrypted_refresh_token"} {
		if columnExists(t, db, "accounts", col) {
			t.Fatalf("accounts.%s should be dropped", col)
		}
	}
	for _, col := range []string{"source_ref", "vcard_raw"} {
		if columnExists(t, db, "contact_records", col) {
			t.Fatalf("contact_records.%s should be dropped", col)
		}
	}

	var localCount, remoteCount, oauthSettings int
	if err := db.QueryRow(`
		SELECT COUNT(*)
		FROM contact_records cr
		JOIN contact_emails ce ON ce.record_id = cr.id
		WHERE cr.id = 'local-rec' AND cr.source = 'local' AND ce.email = 'local@example.com'
	`).Scan(&localCount); err != nil {
		t.Fatalf("count local contact: %v", err)
	}
	if err := db.QueryRow(`SELECT COUNT(*) FROM contact_records WHERE source <> 'local'`).Scan(&remoteCount); err != nil {
		t.Fatalf("count remote contacts: %v", err)
	}
	if err := db.QueryRow(`SELECT COUNT(*) FROM settings WHERE key LIKE 'oauth_active_choice:%'`).Scan(&oauthSettings); err != nil {
		t.Fatalf("count oauth settings: %v", err)
	}
	if localCount != 1 {
		t.Fatalf("local contact should survive v47, got %d", localCount)
	}
	if remoteCount != 0 {
		t.Fatalf("remote contacts should be removed by v47, got %d", remoteCount)
	}
	if oauthSettings != 0 {
		t.Fatalf("oauth_active_choice settings should be removed by v47, got %d", oauthSettings)
	}
}

// TestMigrationV32_LocalRecordIDsRewrittenToUUIDs verifies that migration 32
// transforms "local-<email>" record IDs into canonical UUIDv4s while keeping
// the contact_emails references intact. Simulates the upgrade path for a user
// who applied migration 31 (id format was "local-X@Y") and is now upgrading
// to the schema that uses UUIDs.
func TestMigrationV32_LocalRecordIDsRewrittenToUUIDs(t *testing.T) {
	db := openTestDBAtMigration(t, 31)

	// Seed legacy v31-shape data: a local record with the "local-<email>"
	// synthetic id. Migrate() then applies v32+ in order, including the final
	// v47 local-only schema cleanup.
	if _, err := db.Exec(`
		INSERT INTO contact_records (id, source, kind, fn)
		VALUES ('local-alice@example.com', 'local', 'collected', 'Alice')
	`); err != nil {
		t.Fatalf("seed contact_records: %v", err)
	}
	if _, err := db.Exec(`
		INSERT INTO contact_emails (record_id, email, send_count, is_primary)
		VALUES ('local-alice@example.com', 'alice@example.com', 5, 1)
	`); err != nil {
		t.Fatalf("seed contact_emails: %v", err)
	}

	// Re-run migrations — migration 32 should rewrite the seeded local- id.
	if err := db.Migrate(); err != nil {
		t.Fatalf("re-migrate: %v", err)
	}

	// Record id should now be a UUID (length 36, 4 dashes, hex elsewhere).
	var id string
	if err := db.QueryRow(`SELECT id FROM contact_records WHERE source = 'local'`).Scan(&id); err != nil {
		t.Fatalf("query rewritten id: %v", err)
	}
	if len(id) != 36 {
		t.Errorf("id length = %d, want 36 (UUID)", len(id))
	}
	if id == "local-alice@example.com" {
		t.Errorf("id still has the legacy 'local-' shape: %q", id)
	}

	// contact_emails reference should point at the NEW id, not the old one.
	var refCount, emailCount int
	if err := db.QueryRow(`SELECT COUNT(*) FROM contact_emails WHERE record_id = ?`, id).Scan(&refCount); err != nil {
		t.Fatalf("count refs to new id: %v", err)
	}
	if refCount != 1 {
		t.Errorf("contact_emails row pointing at new id: got %d, want 1", refCount)
	}
	if err := db.QueryRow(`SELECT COUNT(*) FROM contact_emails WHERE record_id = 'local-alice@example.com'`).Scan(&emailCount); err != nil {
		t.Fatalf("count orphan refs: %v", err)
	}
	if emailCount != 0 {
		t.Errorf("contact_emails still references old id: got %d orphan refs", emailCount)
	}

	// Email content + autocomplete metadata are unchanged.
	var email string
	var sendCount int
	if err := db.QueryRow(`SELECT email, send_count FROM contact_emails WHERE record_id = ?`, id).Scan(&email, &sendCount); err != nil {
		t.Fatalf("query preserved fields: %v", err)
	}
	if email != "alice@example.com" {
		t.Errorf("email = %q, want alice@example.com", email)
	}
	if sendCount != 5 {
		t.Errorf("send_count = %d, want 5 (preserved through migration)", sendCount)
	}
}

// TestMigrationV33_AddsAddressbookFK verifies migration 33 cleans existing
// orphans AND wires the new FK so future addressbook deletes cascade to
// state rows automatically.
func TestMigrationV33_AddsAddressbookFK(t *testing.T) {
	db := openTestDBAtMigration(t, 33)

	// Seed a source + addressbook + record so we have a row to chain through.
	if _, err := db.Exec(`
		INSERT INTO contact_sources (id, name, type, url, username, enabled, sync_interval)
		VALUES ('src-1', 'Test', 'carddav', 'https://x', 'u', 1, 60)
	`); err != nil {
		t.Fatalf("seed source: %v", err)
	}
	if _, err := db.Exec(`
		INSERT INTO contact_source_addressbooks (id, source_id, path, name, enabled)
		VALUES ('ab-1', 'src-1', '/dav/', 'ab', 1)
	`); err != nil {
		t.Fatalf("seed addressbook: %v", err)
	}
	if _, err := db.Exec(`
		INSERT INTO contact_records (id, source, source_ref, fn)
		VALUES ('rec-1', 'carddav', 'ab-1', 'Test')
	`); err != nil {
		t.Fatalf("seed record: %v", err)
	}
	if _, err := db.Exec(`
		INSERT INTO carddav_record_state (record_id, addressbook_id, href, etag)
		VALUES ('rec-1', 'ab-1', '/dav/rec-1.vcf', 'etag')
	`); err != nil {
		t.Fatalf("seed state: %v", err)
	}

	// Schema-level invariant: deleting the addressbook now cascades to state.
	// Pre-migration this would have left the state row as a zombie.
	if _, err := db.Exec(`DELETE FROM contact_source_addressbooks WHERE id = 'ab-1'`); err != nil {
		t.Fatalf("delete addressbook: %v", err)
	}

	var stateRows, recordRows int
	if err := db.QueryRow(`SELECT COUNT(*) FROM carddav_record_state WHERE record_id = 'rec-1'`).Scan(&stateRows); err != nil {
		t.Fatalf("count state: %v", err)
	}
	if err := db.QueryRow(`SELECT COUNT(*) FROM contact_records WHERE id = 'rec-1'`).Scan(&recordRows); err != nil {
		t.Fatalf("count records: %v", err)
	}
	if stateRows != 0 {
		t.Errorf("expected 0 state rows after addressbook delete, got %d", stateRows)
	}
	// contact_records does NOT cascade from state (cascade goes the other
	// direction: record→state). So the record row survives. That's expected.
	// (The application-level DeleteSource path handles record cleanup.)
	if recordRows != 1 {
		t.Errorf("expected record to survive addressbook delete (no cascade in that direction); got %d rows", recordRows)
	}
}

// TestMigrationV33_CleansExistingOrphans verifies the pre-step that scrubs
// orphan state rows + records before the FK is added. Simulates the v32
// state by rebuilding carddav_record_state WITHOUT the FK, seeding orphans,
// then re-running migration 33 — which must pre-clean orphans before the
// table rebuild's INSERT.
func TestMigrationV33_CleansExistingOrphans(t *testing.T) {
	db := openTestDBAtMigration(t, 32)

	// Seed: orphan state row whose addressbook doesn't exist. Pre-migration,
	// this insert succeeds because the FK isn't there.
	if _, err := db.Exec(`
		INSERT INTO contact_records (id, source, source_ref, fn)
		VALUES ('zombie-rec', 'carddav', 'dead-ab-id', 'Zombie')
	`); err != nil {
		t.Fatalf("seed zombie record: %v", err)
	}
	if _, err := db.Exec(`
		INSERT INTO carddav_record_state (record_id, addressbook_id, href, etag)
		VALUES ('zombie-rec', 'dead-ab-id', '/dav/zombie.vcf', 'etag')
	`); err != nil {
		t.Fatalf("seed zombie state: %v", err)
	}

	// Seed: contact_records (carddav) with no state row — bloat the
	// migration should drop in pre-step 2.
	if _, err := db.Exec(`
		INSERT INTO contact_records (id, source, fn)
		VALUES ('orphan-record', 'carddav', 'OrphanRec')
	`); err != nil {
		t.Fatalf("seed orphan record: %v", err)
	}

	// Apply just v33; it should pre-clean both rows before rebuilding the table
	// with the FK. Later v47 removes this legacy sidecar entirely, so this test
	// stops at the migration that owns the behavior.
	applyMigrationVersion(t, db, 33)

	var stateCount, recordCount int
	if err := db.QueryRow(`SELECT COUNT(*) FROM carddav_record_state WHERE record_id = 'zombie-rec'`).Scan(&stateCount); err != nil {
		t.Fatalf("count zombie state: %v", err)
	}
	if err := db.QueryRow(`SELECT COUNT(*) FROM contact_records WHERE id IN ('zombie-rec','orphan-record')`).Scan(&recordCount); err != nil {
		t.Fatalf("count orphan records: %v", err)
	}
	if stateCount != 0 {
		t.Errorf("zombie state row should have been pre-cleaned; got %d", stateCount)
	}
	if recordCount != 0 {
		t.Errorf("orphan records (no state) should have been pre-cleaned; got %d", recordCount)
	}

	// FK should now be enforcing — try inserting another orphan, expect FK violation.
	_, err := db.Exec(`
		INSERT INTO contact_records (id, source, fn) VALUES ('post-rec', 'carddav', 'Post');
	`)
	if err != nil {
		t.Fatalf("insert valid record: %v", err)
	}
	_, err = db.Exec(`
		INSERT INTO carddav_record_state (record_id, addressbook_id, href, etag)
		VALUES ('post-rec', 'still-dead', '/dav/post.vcf', 'etag')
	`)
	if err == nil {
		t.Error("expected FK violation when inserting state pointing at dead addressbook_id post-migration, got nil")
	}
}

func TestPath(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.db")
	db, err := Open(path)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	t.Cleanup(func() { db.Close() })

	if got := db.Path(); got != path {
		t.Errorf("Path() = %q, want %q", got, path)
	}
}

// TestMigrationV34_AddsPhotoColumns verifies migration 34 adds the three
// PHOTO columns to contact_records and that existing rows survive with NULL
// values for the new fields.
func TestMigrationV34_AddsPhotoColumns(t *testing.T) {
	db := openTestDB(t)

	// Seed a record before migration 34 (well — migration 34 already ran via
	// openTestDB, but we check that the columns exist + are queryable).
	if _, err := db.Exec(`
		INSERT INTO contact_records (id, source, fn, photo_data, photo_media_type, photo_url)
		VALUES ('rec-1', 'local', 'Alice', 'BASE64DATA', 'image/jpeg', NULL)
	`); err != nil {
		t.Fatalf("insert record with photo: %v", err)
	}

	var photoData, mediaType, photoURL sql.NullString
	if err := db.QueryRow(`
		SELECT photo_data, photo_media_type, photo_url
		FROM contact_records WHERE id = 'rec-1'
	`).Scan(&photoData, &mediaType, &photoURL); err != nil {
		t.Fatalf("read photo columns: %v", err)
	}
	if photoData.String != "BASE64DATA" {
		t.Errorf("photo_data = %q, want BASE64DATA", photoData.String)
	}
	if mediaType.String != "image/jpeg" {
		t.Errorf("photo_media_type = %q, want image/jpeg", mediaType.String)
	}
	if photoURL.Valid {
		t.Errorf("photo_url should be NULL, got %q", photoURL.String)
	}

	// Record with no photo: all three columns NULL.
	if _, err := db.Exec(`
		INSERT INTO contact_records (id, source, fn)
		VALUES ('rec-2', 'local', 'Bob')
	`); err != nil {
		t.Fatalf("insert record without photo: %v", err)
	}
	var dataNull, typeNull, urlNull sql.NullString
	if err := db.QueryRow(`
		SELECT photo_data, photo_media_type, photo_url
		FROM contact_records WHERE id = 'rec-2'
	`).Scan(&dataNull, &typeNull, &urlNull); err != nil {
		t.Fatalf("read NULL photo columns: %v", err)
	}
	if dataNull.Valid || typeNull.Valid || urlNull.Valid {
		t.Errorf("expected all NULL, got %v/%v/%v", dataNull, typeNull, urlNull)
	}
}

// Regression for #289: upgrading from v0.2.5 with a "messy" CardDAV collection
// that lists the same address twice for one vCard must not crash the app inside
// migration 31. The CardDAV backfill collapses fanned-out rows onto one
// record_id; a plain INSERT tripped PRIMARY KEY(record_id, email) and rolled the
// whole startup migration back. INSERT OR IGNORE dedupes instead.
func TestMigration31_DuplicateCardDAVEmailDoesNotFailMigration(t *testing.T) {
	db := openTestDBAtMigration(t, 30)

	// Seed one CardDAV vCard (single href) whose old schema fanned the SAME
	// address out across two rows — the exact #289 trigger.
	if _, err := db.Exec(`
		INSERT INTO contact_sources (id, name, type, url) VALUES ('src1','Test','carddav','https://example.com/dav');
		INSERT INTO contact_source_addressbooks (id, source_id, path) VALUES ('ab1','src1','/');
		INSERT INTO carddav_contacts (id, addressbook_id, email, display_name, href, synced_at) VALUES
			('c1','ab1','dup@example.com','Dup Person','/contacts/1.vcf',CURRENT_TIMESTAMP),
			('c2','ab1','dup@example.com','Dup Person','/contacts/1.vcf',CURRENT_TIMESTAMP);
	`); err != nil {
		t.Fatalf("seed legacy carddav_contacts: %v", err)
	}

	// Apply v31 first so we can assert the backfill dedupes before the final
	// local-only cleanup removes remote rows.
	applyMigrationVersion(t, db, 31)

	// The duplicate collapsed to exactly one contact_emails row.
	var n int
	if err := db.QueryRow(`SELECT COUNT(*) FROM contact_emails WHERE email = 'dup@example.com'`).Scan(&n); err != nil {
		t.Fatalf("count contact_emails: %v", err)
	}
	if n != 1 {
		t.Fatalf("want 1 deduped contact_emails row, got %d", n)
	}

	// Later migrations must still complete, and v47 intentionally discards the
	// remaining remote-contact rows.
	applyMigrationsAfter(t, db, 31)
	if err := db.QueryRow(`SELECT COUNT(*) FROM contact_emails WHERE email = 'dup@example.com'`).Scan(&n); err != nil {
		t.Fatalf("count contact_emails after v47: %v", err)
	}
	if n != 0 {
		t.Fatalf("want remote contact_emails removed by v47, got %d", n)
	}
}
