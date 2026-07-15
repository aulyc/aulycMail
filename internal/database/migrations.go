package database

// Migration represents a database migration
type Migration struct {
	Version int
	SQL     string
}

// migrations is the list of all database migrations
var migrations = []Migration{
	{
		Version: 1,
		SQL: `
			-- Accounts table
			CREATE TABLE accounts (
				id TEXT PRIMARY KEY,
				name TEXT NOT NULL,
				email TEXT NOT NULL UNIQUE,
				
				-- IMAP settings
				imap_host TEXT NOT NULL,
				imap_port INTEGER NOT NULL DEFAULT 993,
				imap_security TEXT NOT NULL DEFAULT 'tls',
				
				-- SMTP settings
				smtp_host TEXT NOT NULL,
				smtp_port INTEGER NOT NULL DEFAULT 587,
				smtp_security TEXT NOT NULL DEFAULT 'starttls',
				
				-- Authentication
				auth_type TEXT NOT NULL DEFAULT 'password',
				username TEXT NOT NULL,
				
				-- State
				enabled INTEGER NOT NULL DEFAULT 1,
				order_index INTEGER NOT NULL DEFAULT 0,
				
				-- Sync settings
				sync_period_days INTEGER NOT NULL DEFAULT 30,
				
				-- Timestamps
				created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
				updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
			);

			-- Sender identities (aliases)
			CREATE TABLE identities (
				id TEXT PRIMARY KEY,
				account_id TEXT NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
				email TEXT NOT NULL,
				name TEXT NOT NULL,
				is_default INTEGER NOT NULL DEFAULT 0,
				signature_html TEXT,
				signature_text TEXT,
				order_index INTEGER NOT NULL DEFAULT 0,
				created_at DATETIME DEFAULT CURRENT_TIMESTAMP
			);

			CREATE INDEX idx_identities_account ON identities(account_id);

			-- Folders table
			CREATE TABLE folders (
				id TEXT PRIMARY KEY,
				account_id TEXT NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
				name TEXT NOT NULL,
				path TEXT NOT NULL,
				folder_type TEXT NOT NULL DEFAULT 'folder',
				parent_id TEXT REFERENCES folders(id) ON DELETE CASCADE,
				
				-- IMAP state
				uid_validity INTEGER,
				uid_next INTEGER,
				highest_mod_seq INTEGER,
				
				-- Counts
				total_count INTEGER DEFAULT 0,
				unread_count INTEGER DEFAULT 0,
				
				-- Sync state
				last_sync DATETIME,
				
				UNIQUE(account_id, path)
			);

			CREATE INDEX idx_folders_account ON folders(account_id);
			CREATE INDEX idx_folders_parent ON folders(parent_id);

			-- Messages table (envelope/header data)
			CREATE TABLE messages (
				id TEXT PRIMARY KEY,
				account_id TEXT NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
				folder_id TEXT NOT NULL REFERENCES folders(id) ON DELETE CASCADE,
				
				-- IMAP identifiers
				uid INTEGER NOT NULL,
				message_id TEXT,
				
				-- Threading
				in_reply_to TEXT,
				thread_id TEXT,
				
				-- Envelope data
				subject TEXT,
				from_name TEXT,
				from_email TEXT,
				to_list TEXT,
				cc_list TEXT,
				bcc_list TEXT,
				reply_to TEXT,
				date DATETIME,
				
				-- Preview
				snippet TEXT,
				
				-- Flags
				is_read INTEGER DEFAULT 0,
				is_starred INTEGER DEFAULT 0,
				is_answered INTEGER DEFAULT 0,
				is_forwarded INTEGER DEFAULT 0,
				is_draft INTEGER DEFAULT 0,
				is_deleted INTEGER DEFAULT 0,
				
				-- Size and attachments
				size INTEGER DEFAULT 0,
				has_attachments INTEGER DEFAULT 0,
				
				-- Body (stored separately for large messages)
				body_text TEXT,
				body_html TEXT,
				
				-- Timestamps
				received_at DATETIME DEFAULT CURRENT_TIMESTAMP,
				
				UNIQUE(folder_id, uid)
			);

			CREATE INDEX idx_messages_account ON messages(account_id);
			CREATE INDEX idx_messages_folder ON messages(folder_id);
			CREATE INDEX idx_messages_date ON messages(date DESC);
			CREATE INDEX idx_messages_thread ON messages(thread_id);
			CREATE INDEX idx_messages_message_id ON messages(message_id);
			CREATE INDEX idx_messages_unread ON messages(folder_id, is_read) WHERE is_read = 0;

			-- Attachments table
			CREATE TABLE attachments (
				id TEXT PRIMARY KEY,
				message_id TEXT NOT NULL REFERENCES messages(id) ON DELETE CASCADE,
				filename TEXT NOT NULL,
				content_type TEXT,
				size INTEGER DEFAULT 0,
				content_id TEXT,
				is_inline INTEGER DEFAULT 0,
				local_path TEXT
			);

			CREATE INDEX idx_attachments_message ON attachments(message_id);

			-- Drafts table (local drafts before sync)
			CREATE TABLE drafts (
				id TEXT PRIMARY KEY,
				account_id TEXT NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
				
				-- Composer state
				to_list TEXT,
				cc_list TEXT,
				bcc_list TEXT,
				subject TEXT,
				body_html TEXT,
				body_text TEXT,
				
				-- Reply context
				in_reply_to_id TEXT,
				reply_type TEXT,
				
				-- Identity
				identity_id TEXT REFERENCES identities(id),
				
				-- Timestamps
				created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
				updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
			);

			CREATE INDEX idx_drafts_account ON drafts(account_id);
		`,
	},
	{
		Version: 2,
		SQL: `
			-- Add encrypted password column for fallback credential storage
			-- Used when OS keyring is not available
			ALTER TABLE accounts ADD COLUMN encrypted_password TEXT;
		`,
	},
	{
		Version: 3,
		SQL: `
			-- Add references column for threading (stores References header as JSON array)
			ALTER TABLE messages ADD COLUMN references_list TEXT;
			
			-- Create index for faster thread lookups
			CREATE INDEX IF NOT EXISTS idx_messages_in_reply_to ON messages(in_reply_to);
		`,
	},
	{
		Version: 4,
		SQL: `
			-- Add sync-related fields to drafts table for local-first draft saving
			
			-- Sync status: pending, synced, failed
			ALTER TABLE drafts ADD COLUMN sync_status TEXT NOT NULL DEFAULT 'pending';
			
			-- IMAP UID when synced (null if not yet synced)
			ALTER TABLE drafts ADD COLUMN imap_uid INTEGER;
			
			-- Folder ID for the drafts folder
			ALTER TABLE drafts ADD COLUMN folder_id TEXT REFERENCES folders(id) ON DELETE SET NULL;
			
			-- References header for threading (JSON array)
			ALTER TABLE drafts ADD COLUMN references_list TEXT;
			
			-- Last sync attempt timestamp
			ALTER TABLE drafts ADD COLUMN last_sync_attempt DATETIME;
			
			-- Sync error message if failed
			ALTER TABLE drafts ADD COLUMN sync_error TEXT;
			
			-- Index for finding pending drafts to sync
			CREATE INDEX IF NOT EXISTS idx_drafts_sync_status ON drafts(sync_status);
		`,
	},
	{
		Version: 5,
		SQL: `
			-- Global settings table for application preferences
			CREATE TABLE IF NOT EXISTS settings (
				key TEXT PRIMARY KEY,
				value TEXT NOT NULL
			);
			
			-- Default read receipt response policy: 'never', 'ask', 'always'
			INSERT INTO settings (key, value) VALUES ('read_receipt_response_policy', 'ask');
			
			-- Per-account read receipt request policy
			-- Controls whether to request read receipts when sending emails
			-- Values: 'never' (default), 'ask', 'always'
			ALTER TABLE accounts ADD COLUMN read_receipt_request_policy TEXT NOT NULL DEFAULT 'never';
			
			-- Read receipt fields on messages
			-- read_receipt_to: Email address that requested the receipt (from Disposition-Notification-To header)
			ALTER TABLE messages ADD COLUMN read_receipt_to TEXT;
			
			-- read_receipt_handled: Whether the user has already responded (sent or ignored)
			ALTER TABLE messages ADD COLUMN read_receipt_handled INTEGER NOT NULL DEFAULT 0;
		`,
	},
	{
		Version: 6,
		SQL: `
			-- Removed remote contact-source schema. Version retained so existing
			-- migration histories stay monotonic.
			SELECT 1;
		`,
	},
	{
		Version: 7,
		SQL: `
			-- Removed remote contact-source credential fallback.
			SELECT 1;
		`,
	},
	{
		Version: 8,
		SQL: `
			-- Add folder mapping columns to accounts table
			-- These allow users to override auto-detected special folders
			-- Empty/NULL means use auto-detection
			ALTER TABLE accounts ADD COLUMN sent_folder_path TEXT;
			ALTER TABLE accounts ADD COLUMN drafts_folder_path TEXT;
			ALTER TABLE accounts ADD COLUMN trash_folder_path TEXT;
			ALTER TABLE accounts ADD COLUMN spam_folder_path TEXT;
			ALTER TABLE accounts ADD COLUMN archive_folder_path TEXT;
			ALTER TABLE accounts ADD COLUMN all_mail_folder_path TEXT;
			ALTER TABLE accounts ADD COLUMN starred_folder_path TEXT;
		`,
	},
	{
		Version: 9,
		SQL: `
			-- Temporary legacy token columns. v47 drops them; they remain here only
			-- because SQLite cannot conditionally drop a column later.
			ALTER TABLE accounts ADD COLUMN encrypted_access_token TEXT;
			ALTER TABLE accounts ADD COLUMN encrypted_refresh_token TEXT;
		`,
	},
	{
		Version: 10,
		SQL: `
			-- Incremental sync support: fetch headers first, bodies later
			-- Add body_fetched column to track whether full body has been downloaded
			-- Default to 1 (true) so existing messages are considered complete
			ALTER TABLE messages ADD COLUMN body_fetched INTEGER NOT NULL DEFAULT 1;

			-- Create index for efficient queries of messages without body
			-- Used during background body fetching
			CREATE INDEX IF NOT EXISTS idx_messages_body_fetched ON messages(folder_id, body_fetched);
		`,
	},
	{
		Version: 11,
		SQL: `
			-- Add sync_interval column to accounts for automatic email polling
			-- Default to 30 minutes. Value of 0 means manual sync only.
			-- This controls how often the app checks for new mail via polling.
			-- IMAP IDLE (push) is used when available for real-time notifications.
			ALTER TABLE accounts ADD COLUMN sync_interval INTEGER NOT NULL DEFAULT 30;
		`,
	},
	{
		Version: 12,
		SQL: `
			-- Add color column to accounts for visual identification in unified inbox
			-- Each account can have a unique color shown as a dot indicator
			ALTER TABLE accounts ADD COLUMN color TEXT NOT NULL DEFAULT '';
		`,
	},
	{
		Version: 13,
		SQL: `
			-- App state table for persisting UI state across sessions
			-- Uses a key-value design for flexibility in storing various state data
			CREATE TABLE IF NOT EXISTS app_state (
				key TEXT PRIMARY KEY,
				value TEXT NOT NULL,
				updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
			);
		`,
	},
	{
		Version: 14,
		SQL: `
			-- Create FTS5 virtual table for full-text search
			-- Uses content= to create an "external content" table that shadows messages
			-- This avoids duplicating data while enabling fast full-text search
			CREATE VIRTUAL TABLE messages_fts USING fts5(
				subject,
				from_name,
				from_email,
				to_list,
				cc_list,
				snippet,
				body_text,
				content='messages',
				content_rowid='rowid'
			);

			-- Triggers to keep FTS in sync with messages table
			-- These fire on INSERT/UPDATE/DELETE to maintain index consistency
			
			CREATE TRIGGER messages_fts_insert AFTER INSERT ON messages BEGIN
				INSERT INTO messages_fts(rowid, subject, from_name, from_email, to_list, cc_list, snippet, body_text)
				VALUES (NEW.rowid, NEW.subject, NEW.from_name, NEW.from_email, NEW.to_list, NEW.cc_list, NEW.snippet, NEW.body_text);
			END;

			CREATE TRIGGER messages_fts_delete AFTER DELETE ON messages BEGIN
				INSERT INTO messages_fts(messages_fts, rowid, subject, from_name, from_email, to_list, cc_list, snippet, body_text)
				VALUES ('delete', OLD.rowid, OLD.subject, OLD.from_name, OLD.from_email, OLD.to_list, OLD.cc_list, OLD.snippet, OLD.body_text);
			END;

			CREATE TRIGGER messages_fts_update AFTER UPDATE ON messages BEGIN
				INSERT INTO messages_fts(messages_fts, rowid, subject, from_name, from_email, to_list, cc_list, snippet, body_text)
				VALUES ('delete', OLD.rowid, OLD.subject, OLD.from_name, OLD.from_email, OLD.to_list, OLD.cc_list, OLD.snippet, OLD.body_text);
				INSERT INTO messages_fts(rowid, subject, from_name, from_email, to_list, cc_list, snippet, body_text)
				VALUES (NEW.rowid, NEW.subject, NEW.from_name, NEW.from_email, NEW.to_list, NEW.cc_list, NEW.snippet, NEW.body_text);
			END;

			-- Track indexing status per folder for background indexing progress
			-- This allows the UI to show indexing progress and warn users if search
			-- results may be incomplete
			CREATE TABLE fts_index_status (
				folder_id TEXT PRIMARY KEY REFERENCES folders(id) ON DELETE CASCADE,
				indexed_count INTEGER DEFAULT 0,
				total_count INTEGER DEFAULT 0,
				is_complete INTEGER DEFAULT 0,
				last_indexed_at DATETIME
			);
		`,
	},
	{
		Version: 15,
		SQL: `
			-- Add signature settings to identities table
			-- These columns control signature behavior per identity

			-- Master toggle for signature (default: enabled)
			ALTER TABLE identities ADD COLUMN signature_enabled INTEGER NOT NULL DEFAULT 1;

			-- When to append signature (default: all enabled)
			ALTER TABLE identities ADD COLUMN signature_for_new INTEGER NOT NULL DEFAULT 1;
			ALTER TABLE identities ADD COLUMN signature_for_reply INTEGER NOT NULL DEFAULT 1;
			ALTER TABLE identities ADD COLUMN signature_for_forward INTEGER NOT NULL DEFAULT 1;

			-- Signature placement in replies/forwards: 'above' or 'below' quoted text
			ALTER TABLE identities ADD COLUMN signature_placement TEXT NOT NULL DEFAULT 'above';

			-- Whether to add "-- " separator before signature (default: off)
			ALTER TABLE identities ADD COLUMN signature_separator INTEGER NOT NULL DEFAULT 0;

			-- Updated timestamp for identities (NULL default, set by application code)
			ALTER TABLE identities ADD COLUMN updated_at DATETIME;
		`,
	},
	{
		Version: 16,
		SQL: `
			-- Image allowlist table for "Always Load" remote images feature
			-- Allows users to trust specific senders or domains to auto-load images
			-- type: 'domain' (e.g., 'company.com') or 'sender' (e.g., 'newsletter@company.com')
			CREATE TABLE IF NOT EXISTS image_allowlist (
				id INTEGER PRIMARY KEY AUTOINCREMENT,
				type TEXT NOT NULL CHECK(type IN ('domain', 'sender')),
				value TEXT NOT NULL COLLATE NOCASE,
				created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
				UNIQUE(type, value)
			);

			CREATE INDEX idx_image_allowlist_type_value ON image_allowlist(type, value);
		`,
	},
	{
		Version: 17,
		SQL: `
			-- Removed remote contact-source OAuth metadata.
			SELECT 1;
		`,
	},
	{
		Version: 18,
		SQL: `
				-- Trusted certificates table for certificate trust-on-first-use (TOFU)
				-- Trust is scoped to the host that the user accepted.
				CREATE TABLE IF NOT EXISTS trusted_certificates (
					id TEXT PRIMARY KEY,
					fingerprint TEXT NOT NULL,
					host TEXT NOT NULL DEFAULT '',
					subject TEXT NOT NULL,
					issuer TEXT NOT NULL,
					not_before DATETIME,
					not_after DATETIME,
					accepted_at DATETIME DEFAULT CURRENT_TIMESTAMP,
					UNIQUE(host, fingerprint)
				);
			`,
	},
	{
		Version: 19,
		SQL: `
			-- Legacy reserved S/MIME certificate schema retained for compatibility
			CREATE TABLE IF NOT EXISTS smime_certificates (
				id TEXT PRIMARY KEY,
				account_id TEXT NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
				email TEXT NOT NULL,
				subject TEXT NOT NULL,
				issuer TEXT NOT NULL,
				serial_number TEXT NOT NULL,
				fingerprint TEXT NOT NULL UNIQUE,
				not_before DATETIME NOT NULL,
				not_after DATETIME NOT NULL,
				cert_chain_pem TEXT NOT NULL,
				encrypted_private_key TEXT,
				is_default INTEGER NOT NULL DEFAULT 0,
				created_at DATETIME DEFAULT CURRENT_TIMESTAMP
			);

			CREATE INDEX IF NOT EXISTS idx_smime_certificates_account ON smime_certificates(account_id);
			CREATE INDEX IF NOT EXISTS idx_smime_certificates_email ON smime_certificates(email);

			-- Legacy reserved sender certificate schema retained for compatibility
			CREATE TABLE IF NOT EXISTS smime_sender_certs (
				id TEXT PRIMARY KEY,
				email TEXT NOT NULL,
				subject TEXT NOT NULL,
				issuer TEXT NOT NULL,
				serial_number TEXT NOT NULL,
				fingerprint TEXT NOT NULL UNIQUE,
				not_before DATETIME NOT NULL,
				not_after DATETIME NOT NULL,
				cert_pem TEXT NOT NULL,
				collected_at DATETIME DEFAULT CURRENT_TIMESTAMP,
				last_seen_at DATETIME DEFAULT CURRENT_TIMESTAMP
			);

			CREATE INDEX IF NOT EXISTS idx_smime_sender_certs_email ON smime_sender_certs(email);
			CREATE INDEX IF NOT EXISTS idx_smime_sender_certs_fingerprint ON smime_sender_certs(fingerprint);

			-- Legacy reserved verification-result columns
			ALTER TABLE messages ADD COLUMN smime_status TEXT;
			ALTER TABLE messages ADD COLUMN smime_signer_email TEXT;
			ALTER TABLE messages ADD COLUMN smime_signer_subject TEXT;

			-- Legacy reserved per-account signing policy
			ALTER TABLE accounts ADD COLUMN smime_sign_policy TEXT NOT NULL DEFAULT 'never';
			ALTER TABLE accounts ADD COLUMN smime_default_cert_id TEXT;
		`,
	},
	{
		Version: 20,
		SQL: `
			-- Legacy reserved raw-message body column
			ALTER TABLE messages ADD COLUMN smime_raw_body BLOB;

			-- Legacy reserved encrypted-message flag
			ALTER TABLE messages ADD COLUMN smime_encrypted INTEGER NOT NULL DEFAULT 0;

			-- Legacy reserved per-account encryption policy
			ALTER TABLE accounts ADD COLUMN smime_encrypt_policy TEXT NOT NULL DEFAULT 'never';
		`,
	},
	{
		Version: 21,
		SQL: `
			-- Legacy reserved encrypted-draft flag
			ALTER TABLE drafts ADD COLUMN encrypted INTEGER NOT NULL DEFAULT 0;

			-- Legacy reserved encrypted-draft payload
			ALTER TABLE drafts ADD COLUMN encrypted_body BLOB;
		`,
	},
	{
		Version: 22,
		SQL: `
			-- Legacy reserved per-message signing preference
			ALTER TABLE drafts ADD COLUMN sign_message INTEGER NOT NULL DEFAULT 0;
		`,
	},
	{
		Version: 23,
		SQL: `
			-- Store attachment data alongside draft body (inline images + regular attachments)
			-- JSON-serialized []smtp.Attachment for regular drafts
			-- Legacy encrypted-draft rows kept attachments in encrypted_body.
			ALTER TABLE drafts ADD COLUMN attachments_data BLOB;
		`,
	},
	{
		Version: 24,
		SQL: `
			-- Legacy reserved PGP key schema retained for compatibility
			CREATE TABLE IF NOT EXISTS pgp_keys (
				id TEXT PRIMARY KEY,
				account_id TEXT NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
				email TEXT NOT NULL,
				key_id TEXT NOT NULL,
				fingerprint TEXT NOT NULL UNIQUE,
				user_id TEXT NOT NULL,
				algorithm TEXT NOT NULL,
				key_size INTEGER,
				created_at_key DATETIME,
				expires_at_key DATETIME,
				public_key_armored TEXT NOT NULL,
				encrypted_private_key TEXT,
				is_default INTEGER NOT NULL DEFAULT 0,
				created_at DATETIME DEFAULT CURRENT_TIMESTAMP
			);
			CREATE INDEX IF NOT EXISTS idx_pgp_keys_account ON pgp_keys(account_id);
			CREATE INDEX IF NOT EXISTS idx_pgp_keys_email ON pgp_keys(email);
			CREATE INDEX IF NOT EXISTS idx_pgp_keys_fingerprint ON pgp_keys(fingerprint);

			-- Legacy reserved collected sender public-key schema
			CREATE TABLE IF NOT EXISTS pgp_sender_keys (
				id TEXT PRIMARY KEY,
				email TEXT NOT NULL,
				key_id TEXT NOT NULL,
				fingerprint TEXT NOT NULL UNIQUE,
				user_id TEXT NOT NULL,
				algorithm TEXT NOT NULL,
				key_size INTEGER,
				created_at_key DATETIME,
				expires_at_key DATETIME,
				public_key_armored TEXT NOT NULL,
				source TEXT NOT NULL DEFAULT 'message',
				collected_at DATETIME DEFAULT CURRENT_TIMESTAMP,
				last_seen_at DATETIME DEFAULT CURRENT_TIMESTAMP
			);
			CREATE INDEX IF NOT EXISTS idx_pgp_sender_keys_email ON pgp_sender_keys(email);
			CREATE INDEX IF NOT EXISTS idx_pgp_sender_keys_fingerprint ON pgp_sender_keys(fingerprint);

			-- Legacy reserved message PGP columns
			ALTER TABLE messages ADD COLUMN pgp_status TEXT;
			ALTER TABLE messages ADD COLUMN pgp_signer_email TEXT;
			ALTER TABLE messages ADD COLUMN pgp_signer_key_id TEXT;
			ALTER TABLE messages ADD COLUMN pgp_raw_body BLOB;
			ALTER TABLE messages ADD COLUMN pgp_encrypted INTEGER NOT NULL DEFAULT 0;

			-- Legacy reserved account PGP policies
			ALTER TABLE accounts ADD COLUMN pgp_sign_policy TEXT NOT NULL DEFAULT 'never';
			ALTER TABLE accounts ADD COLUMN pgp_encrypt_policy TEXT NOT NULL DEFAULT 'never';
			ALTER TABLE accounts ADD COLUMN pgp_default_key_id TEXT;

			-- Legacy reserved draft PGP fields
			ALTER TABLE drafts ADD COLUMN pgp_sign_message INTEGER NOT NULL DEFAULT 0;
			ALTER TABLE drafts ADD COLUMN pgp_encrypted INTEGER NOT NULL DEFAULT 0;
			ALTER TABLE drafts ADD COLUMN pgp_encrypted_body BLOB;
		`,
	},
	{
		Version: 25,
		SQL: `
			-- Legacy reserved PGP key-server table retained for compatibility
			CREATE TABLE IF NOT EXISTS pgp_keyservers (
				id INTEGER PRIMARY KEY AUTOINCREMENT,
				url TEXT NOT NULL UNIQUE,
				order_index INTEGER NOT NULL DEFAULT 0,
				created_at DATETIME DEFAULT CURRENT_TIMESTAMP
			);

			INSERT OR IGNORE INTO pgp_keyservers (url, order_index) VALUES
				('https://keys.openpgp.org', 0),
				('https://keyserver.ubuntu.com', 1),
				('https://pgp.mit.edu', 2);
		`,
	},
	{
		Version: 26,
		SQL: `
			ALTER TABLE accounts ADD COLUMN sync_all_folders INTEGER NOT NULL DEFAULT 0;
			ALTER TABLE folders ADD COLUMN subscribed INTEGER NOT NULL DEFAULT 0;
		`,
	},
	{
		Version: 27,
		SQL:     `ALTER TABLE accounts ADD COLUMN sync_folders_enabled INTEGER NOT NULL DEFAULT 0;`,
	},
	{
		Version: 28,
		SQL:     `ALTER TABLE accounts ADD COLUMN shared_mailbox_parent_id TEXT DEFAULT NULL;`,
	},
	{
		Version: 29,
		SQL: `
			-- Removed OAuth token schema.
			SELECT 1;
		`,
	},
	{
		Version: 30,
		SQL: `
			-- Removed remote contact-source write capability flag.
			SELECT 1;
		`,
	},
	{
		Version: 31,
		SQL: `
			-- Phase 2b.2.a: Unified contact-record schema.
			--
			-- This migration replaces the legacy denormalized "contacts"
			-- (autocomplete-by-email-only) table with a local record-based shape:
			--
			--   contact_records      → one row per logical local contact
			--   contact_emails       → composite-PK (record_id, email) with per-email
			--                          autocomplete metadata (send_count, last_used,
			--                          name_overridden). Replaces the legacy contacts
			--                          table as the autocomplete index.
			--   contact_phones       → multi-value per record
			--   contact_addresses    → multi-value per record (structured parts)
			--   contact_urls         → multi-value per record
			--   contact_impps        → multi-value per record (instant messaging)
			--   contact_categories   → multi-value per record (tags)
			--
			-- This migration is the architectural pivot for Contacts:
			-- - Mail's autocomplete still works through contact.Store's public API
			--   (Search/AddOrUpdate/Get) — only the internals change to query the
			--   unified tables.
			-- - Multi-field local contacts land (phone/address/org/etc.).

			-- Defensive: contact.Store.ensureTable creates "contacts" lazily AFTER
			-- migrations run, so on a fresh install the table won't exist here. Make
			-- sure it exists so the backfill SELECTs below don't error.
			--
			-- Shape matches the LEGACY (pre-v0.3.0) ensureTable schema — no
			-- kind, no name_overridden. Older aulycmail installs (≤ v0.2.4) have
			-- the table in this shape, so referencing those columns in the
			-- backfill SELECTs would fail on real production DBs. Defaults
			-- for the post-migration columns are supplied as literals in the
			-- INSERTs below.
			CREATE TABLE IF NOT EXISTS contacts (
				email TEXT PRIMARY KEY,
				display_name TEXT,
				send_count INTEGER DEFAULT 0,
				last_used DATETIME,
				created_at DATETIME DEFAULT CURRENT_TIMESTAMP
			);

			-- New tables.

			CREATE TABLE contact_records (
				id            TEXT PRIMARY KEY,
				source        TEXT NOT NULL,             -- 'local'
				kind          TEXT,                      -- 'manual' | 'collected'
				source_ref    TEXT,
				fn            TEXT,                      -- vCard FN (display name)
				n_given       TEXT,                      -- vCard N: given name
				n_family      TEXT,                      -- vCard N: family name
				org           TEXT,
				title         TEXT,
				note          TEXT,
				bday          TEXT,                      -- ISO-8601 date string (vCard BDAY)
				nickname      TEXT,
				vcard_raw     TEXT,
				created_at    DATETIME DEFAULT CURRENT_TIMESTAMP,
				updated_at    DATETIME DEFAULT CURRENT_TIMESTAMP
			);

			CREATE INDEX idx_contact_records_source ON contact_records(source);
			CREATE INDEX idx_contact_records_source_kind ON contact_records(source, kind);

			CREATE TABLE contact_emails (
				record_id       TEXT NOT NULL REFERENCES contact_records(id) ON DELETE CASCADE,
				email           TEXT NOT NULL,                  -- normalized lowercase
				email_type      TEXT,                           -- vCard TYPE param: 'home', 'work', 'internet', etc.
				is_primary      INTEGER NOT NULL DEFAULT 0,
				send_count      INTEGER NOT NULL DEFAULT 0,     -- per-email autocomplete ranking
				last_used       DATETIME,
				name_overridden INTEGER NOT NULL DEFAULT 0,     -- preserves user-edited fn across auto-collection
				PRIMARY KEY (record_id, email)
			);

			CREATE INDEX idx_contact_emails_email ON contact_emails(email);
			CREATE INDEX idx_contact_emails_rank ON contact_emails(send_count DESC, last_used DESC);

			CREATE TABLE contact_phones (
				record_id   TEXT NOT NULL REFERENCES contact_records(id) ON DELETE CASCADE,
				number      TEXT NOT NULL,
				phone_type  TEXT,
				is_primary  INTEGER NOT NULL DEFAULT 0,
				PRIMARY KEY (record_id, number)
			);

			CREATE TABLE contact_addresses (
				record_id   TEXT NOT NULL REFERENCES contact_records(id) ON DELETE CASCADE,
				addr_type   TEXT,                       -- 'home', 'work', etc.
				street      TEXT,
				city        TEXT,
				region      TEXT,
				postcode    TEXT,
				country     TEXT,
				-- No natural PK; allow duplicates and let app sort it out
				idx         INTEGER NOT NULL DEFAULT 0  -- ordinal for stable display order
			);

			CREATE INDEX idx_contact_addresses_record ON contact_addresses(record_id);

			CREATE TABLE contact_urls (
				record_id   TEXT NOT NULL REFERENCES contact_records(id) ON DELETE CASCADE,
				url         TEXT NOT NULL,
				url_type    TEXT,
				PRIMARY KEY (record_id, url)
			);

			CREATE TABLE contact_impps (
				record_id   TEXT NOT NULL REFERENCES contact_records(id) ON DELETE CASCADE,
				handle      TEXT NOT NULL,            -- e.g. xmpp:user@host
				impp_type   TEXT,
				PRIMARY KEY (record_id, handle)
			);

			CREATE TABLE contact_categories (
				record_id   TEXT NOT NULL REFERENCES contact_records(id) ON DELETE CASCADE,
				category    TEXT NOT NULL,
				PRIMARY KEY (record_id, category)
			);

			-- Backfill from legacy contacts. One record per row (email is the natural
			-- record-grain for local contacts today; multi-field expansion for local
			-- happens via the new sub-tables which start empty).
			-- record id: derived from email so subsequent linking via record_id is
			-- stable. Older aulycmail never exposed contact ids externally; this just
			-- needs to be unique + deterministic within the migration.
			-- kind / name_overridden are hardcoded literals here rather than
			-- selected from the contacts table because legacy v0.2.x DBs
			-- don't have those columns (the legacy ensureTable shape never
			-- included them). Semantic match: legacy local contacts were
			-- exclusively auto-collected from sent mail (no manual-add UI
			-- before v0.3.0), and name_overridden was never set, so
			-- 'collected' / 0 are the correct historical defaults.
			INSERT INTO contact_records (id, source, kind, fn, created_at, updated_at)
			SELECT
				'local-' || email,
				'local',
				'collected',
				display_name,
				created_at,
				created_at
			FROM contacts;

			-- OR IGNORE: legacy data can hold case/whitespace variants that normalize
			-- to the same value. Dropping the duplicate is correct.
			INSERT OR IGNORE INTO contact_emails (record_id, email, send_count, last_used, name_overridden, is_primary)
			SELECT
				'local-' || email,
				email,
				send_count,
				last_used,
				0,
				1
			FROM contacts;

			-- Drop legacy table — the unified schema is now authoritative.
			DROP TABLE contacts;
		`,
	},
	{
		Version: 32,
		SQL: `
			-- Phase 2b.2 follow-up: rewrite local contact_records IDs from the
			-- synthetic "local-<email>" form into real UUIDv4s.
			--
			-- Why: local records got the "local-<email>" shape in migration 31 as
			-- a leftover from the legacy contacts(email PK) schema. That blocks
			-- email editing and keeps the record identity tied to one address.
			--
			-- This migration unifies the identity shape: every contact_records
			-- row gets a UUID, and the EMAIL becomes a fully-editable sub-row in
			-- contact_emails. The multi-field Edit/Create UIs landing in 2b.2.b
			-- and 2b.2.c then design once for both sources.
			--
			-- Implementation: SQLite's randomblob() generates a fresh value per
			-- call, so each row gets its own UUID. PRAGMA defer_foreign_keys
			-- lets us update contact_records.id and the dependent
			-- contact_emails.record_id (plus sub-tables) without intermediate
			-- FK violations — the check is deferred to commit time.

			PRAGMA defer_foreign_keys = ON;

			-- Build the old-id → new-uuid mapping. One row per source='local'
			-- record. The UUID format is canonical 8-4-4-4-12 hex with version
			-- nibble 4 and variant nibble 8/9/a/b — matches RFC 4122 UUIDv4.
			CREATE TEMPORARY TABLE _migration_32_idmap (
				old_id TEXT PRIMARY KEY,
				new_id TEXT NOT NULL UNIQUE
			);

			INSERT INTO _migration_32_idmap (old_id, new_id)
			SELECT
				id,
				lower(
					substr(hex(randomblob(4)), 1, 8) || '-' ||
					substr(hex(randomblob(2)), 1, 4) || '-4' ||
					substr(hex(randomblob(2)), 2, 3) || '-' ||
					substr('89ab', 1 + (abs(random()) % 4), 1) ||
					substr(hex(randomblob(2)), 2, 3) || '-' ||
					substr(hex(randomblob(6)), 1, 12)
				)
			FROM contact_records
			WHERE source = 'local';

			-- Apply the new IDs to contact_records and every sub-table that
			-- references record_id.
			UPDATE contact_records
			SET id = (SELECT new_id FROM _migration_32_idmap WHERE old_id = contact_records.id)
			WHERE id IN (SELECT old_id FROM _migration_32_idmap);

			UPDATE contact_emails
			SET record_id = (SELECT new_id FROM _migration_32_idmap WHERE old_id = contact_emails.record_id)
			WHERE record_id IN (SELECT old_id FROM _migration_32_idmap);

			UPDATE contact_phones
			SET record_id = (SELECT new_id FROM _migration_32_idmap WHERE old_id = contact_phones.record_id)
			WHERE record_id IN (SELECT old_id FROM _migration_32_idmap);

			UPDATE contact_addresses
			SET record_id = (SELECT new_id FROM _migration_32_idmap WHERE old_id = contact_addresses.record_id)
			WHERE record_id IN (SELECT old_id FROM _migration_32_idmap);

			UPDATE contact_urls
			SET record_id = (SELECT new_id FROM _migration_32_idmap WHERE old_id = contact_urls.record_id)
			WHERE record_id IN (SELECT old_id FROM _migration_32_idmap);

			UPDATE contact_impps
			SET record_id = (SELECT new_id FROM _migration_32_idmap WHERE old_id = contact_impps.record_id)
			WHERE record_id IN (SELECT old_id FROM _migration_32_idmap);

			UPDATE contact_categories
			SET record_id = (SELECT new_id FROM _migration_32_idmap WHERE old_id = contact_categories.record_id)
			WHERE record_id IN (SELECT old_id FROM _migration_32_idmap);

			DROP TABLE _migration_32_idmap;
		`,
	},
	{
		Version: 33,
		SQL: `
			-- Removed remote contact sidecar schema.
			SELECT 1;
		`,
	},
	{
		Version: 34,
		SQL: `
			-- Phase 2b.2.b.2: first-class PHOTO field support on contact_records.
			--
			-- Before this migration, PHOTO data was not extracted, displayed, or
			-- editable. This migration adds three columns so the parser
			-- can land photos natively and the builder can emit them under
			-- explicit control.
			--
			-- Invariant: at most one of {photo_data + photo_media_type} OR
			-- photo_url is populated per row. NULL across all three = no photo.
			-- Inline (base64) is the common CardDAV shape (Nextcloud, iCloud,
			-- Apple). URL refs are rarer; we parse + store them but don't
			-- fetch in this phase. Write path always emits inline.

			ALTER TABLE contact_records ADD COLUMN photo_data TEXT;
			ALTER TABLE contact_records ADD COLUMN photo_media_type TEXT;
			ALTER TABLE contact_records ADD COLUMN photo_url TEXT;
		`,
	},
	{
		Version: 35,
		SQL: `
			-- Removed extension secret registry.
			SELECT 1;
		`,
	},
	{
		Version: 36,
		SQL: `
			-- Removed OAuth encrypted-token fallback.
			SELECT 1;
		`,
	},
	{
		Version: 37,
		SQL: `
			-- v0.3.0: "No outgoing server" + separate SMTP credentials.
			--
			-- no_outgoing_server: marks the account as receive-only. SMTP
			-- host/port/security are ignored when set; the composer hides
			-- the account (and all its identities) from the From dropdown.
			--
			-- smtp_username: SMTP-specific username when the user supplies
			-- separate SMTP credentials. Empty (the default for every
			-- pre-v0.3.0 row) preserves legacy behavior — SMTP reuses the
			-- account's Username + IMAP keyring password. Non-empty signals
			-- the SMTP send path to use this username + a separately-stored
			-- password keyed at "<accountID>:smtp" in the keyring.

			ALTER TABLE accounts ADD COLUMN no_outgoing_server INTEGER NOT NULL DEFAULT 0;
			ALTER TABLE accounts ADD COLUMN smtp_username TEXT NOT NULL DEFAULT '';
			-- Encrypted-DB fallback for the SMTP-specific password when the
			-- keyring is unavailable. Mirrors encrypted_password's role for
			-- IMAP. Only consulted when smtp_username != ''.
			ALTER TABLE accounts ADD COLUMN encrypted_smtp_password TEXT;
		`,
	},
	{
		Version: 38,
		SQL: `
			-- v0.3.0: "Reply/Forward with" identity preference for receive-only
			-- accounts. Stores the identity ID to pre-select in the composer
			-- when replying or forwarding a message received via a
			-- no_outgoing_server account. Empty (the default) falls back to
			-- the user's default sending account, then to the first available
			-- identity. Only consulted when no_outgoing_server = 1; sendable
			-- accounts use their own identities directly.

			ALTER TABLE accounts ADD COLUMN reply_forward_identity_id TEXT NOT NULL DEFAULT '';
		`,
	},
	{
		Version: 39,
		SQL: `
			-- Persistent flag set when a body fetch+parse produced no usable content
			-- (and the message isn't encrypted, which is intentionally empty until
			-- decrypted at view time). Replaces the previous in-memory
			-- failedParseAttempts map in sync/fetch.go that reset every sync session
			-- — without persistence, an unparseable message was re-fetched from IMAP
			-- on every sync cycle forever. A future parser improvement clears this
			-- flag via its own migration so previously-skipped messages get retried
			-- under the new parser.

			ALTER TABLE messages ADD COLUMN body_failed INTEGER NOT NULL DEFAULT 0;
		`,
	},
	{
		Version: 40,
		SQL: `
			-- Role flags for auto-collected contacts. A collected contact can play
			-- multiple roles across the user's mail, so each role is an independent
			-- boolean rather than a single 'kind'. Drives the Contacts sidebar's
			-- 全部 / 发件人 / 收件人 / 抄送密送 categories:
			--   collected_sender    — appeared as the From of a received message
			--   collected_recipient — was a To recipient of a message the user sent
			--   collected_ccbcc     — was a Cc/Bcc recipient of a sent message
			-- Manual contacts leave all three at 0. Existing collected rows were
			-- collected from received mail's From, so backfill them as senders.

			ALTER TABLE contact_records ADD COLUMN collected_sender INTEGER NOT NULL DEFAULT 0;
			ALTER TABLE contact_records ADD COLUMN collected_recipient INTEGER NOT NULL DEFAULT 0;
			ALTER TABLE contact_records ADD COLUMN collected_ccbcc INTEGER NOT NULL DEFAULT 0;

			UPDATE contact_records SET collected_sender = 1 WHERE source = 'local' AND kind = 'collected';
		`,
	},
	{
		Version: 41,
		SQL: `
			-- Split the combined Cc/Bcc role into separate Cc and Bcc flags so the
			-- Contacts sidebar can show 抄送 and 密送 as distinct categories.
			-- The old combined flag (collected_ccbcc) couldn't tell Cc from Bcc
			-- retroactively, so backfill existing combined rows as Cc; a "refresh
			-- contacts from mail" re-scan re-classifies them precisely afterward.

			ALTER TABLE contact_records ADD COLUMN collected_cc INTEGER NOT NULL DEFAULT 0;
			ALTER TABLE contact_records ADD COLUMN collected_bcc INTEGER NOT NULL DEFAULT 0;

			UPDATE contact_records SET collected_cc = 1 WHERE collected_ccbcc = 1;
			`,
	},
	{
		Version: 42,
		SQL: `
				-- Scope TOFU certificate trust to the accepted host instead of making
				-- a fingerprint globally trusted for every IMAP/SMTP endpoint.
				CREATE TABLE trusted_certificates_new (
					id TEXT PRIMARY KEY,
					fingerprint TEXT NOT NULL,
					host TEXT NOT NULL DEFAULT '',
					subject TEXT NOT NULL,
					issuer TEXT NOT NULL,
					not_before DATETIME,
					not_after DATETIME,
					accepted_at DATETIME DEFAULT CURRENT_TIMESTAMP,
					UNIQUE(host, fingerprint)
				);

				INSERT OR REPLACE INTO trusted_certificates_new
					(id, fingerprint, host, subject, issuer, not_before, not_after, accepted_at)
				SELECT
					id, fingerprint, LOWER(TRIM(COALESCE(host, ''))), subject, issuer, not_before, not_after, accepted_at
				FROM trusted_certificates;

				DROP TABLE trusted_certificates;
				ALTER TABLE trusted_certificates_new RENAME TO trusted_certificates;
				CREATE INDEX IF NOT EXISTS idx_trusted_certificates_host ON trusted_certificates(host);
			`,
	},
	{
		Version: 43,
		SQL: `
			-- Signature separator style. Keeps the old signature_separator boolean
			-- for compatibility, while allowing explicit separator text choices.
			-- Existing enabled separators become the new five-dash separator.
			ALTER TABLE identities ADD COLUMN signature_separator_style TEXT NOT NULL DEFAULT '';
			UPDATE identities
			SET signature_separator_style = '-----'
			WHERE signature_separator = 1 AND signature_separator_style = '';
		`,
	},
	{
		Version: 44,
		SQL: `
			-- Local compose state for source messages. This intentionally lives
			-- outside messages so IMAP flag refreshes cannot erase app-local
			-- "has draft" / "sent reply or forward" indicators.
			ALTER TABLE drafts ADD COLUMN source_message_id TEXT REFERENCES messages(id) ON DELETE SET NULL;

			CREATE TABLE IF NOT EXISTS message_compose_status (
				source_message_id TEXT PRIMARY KEY REFERENCES messages(id) ON DELETE CASCADE,
				action_type TEXT NOT NULL CHECK (action_type IN ('reply', 'reply-all', 'forward')),
				status TEXT NOT NULL CHECK (status IN ('draft', 'sent')),
				draft_id TEXT REFERENCES drafts(id) ON DELETE SET NULL,
				updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
			);

			CREATE INDEX IF NOT EXISTS idx_message_compose_status_status ON message_compose_status(status);
			CREATE INDEX IF NOT EXISTS idx_message_compose_status_draft ON message_compose_status(draft_id);
		`,
	},
	{
		Version: 45,
		SQL: `
			-- Split the old "sync period" into explicit retention, daily sync,
			-- full-check, and body-download settings.
			--
			-- local_retention_days:
			--   0 keeps all local messages for archive/backup; positive values
			--   preserve the old "delete local messages older than N days" behavior.
			--
			-- sync_strategy:
			--   incremental performs fast UIDNEXT/highest-UID checks between
			--   scheduled full validations; full preserves the old every-sync
			--   complete UID SEARCH behavior.
			--
			-- body_download_policy:
			--   on_demand skips background body downloads; recent downloads only
			--   body_download_days; all downloads every missing body.
			ALTER TABLE accounts ADD COLUMN local_retention_days INTEGER NOT NULL DEFAULT 0;
			ALTER TABLE accounts ADD COLUMN sync_strategy TEXT NOT NULL DEFAULT 'incremental';
			ALTER TABLE accounts ADD COLUMN full_check_interval_days INTEGER NOT NULL DEFAULT 7;
			ALTER TABLE accounts ADD COLUMN body_download_policy TEXT NOT NULL DEFAULT 'on_demand';
			ALTER TABLE accounts ADD COLUMN body_download_days INTEGER NOT NULL DEFAULT 180;
			ALTER TABLE folders ADD COLUMN last_full_sync DATETIME;

			-- Existing accounts keep their local-retention behavior from the old
			-- combined setting. Body-download behavior maps old "all" to all
			-- content; limited old ranges move to the new fixed recent range.
			UPDATE accounts
			SET local_retention_days = CASE
				WHEN sync_period_days < 0 THEN 30
				ELSE sync_period_days
			END;

			UPDATE accounts
			SET
				body_download_policy = CASE
					WHEN sync_period_days = 0 THEN 'all'
					ELSE 'recent'
				END,
				body_download_days = 180;
		`,
	},
	{
		Version: 46,
		SQL: `
			-- Narrow FTS maintenance to columns that are actually indexed.
			-- Flag-only updates are hot during sync and must not re-tokenize
			-- subject/body text or rewrite FTS index pages.
			DROP TRIGGER IF EXISTS messages_fts_update;
			CREATE TRIGGER messages_fts_update
			AFTER UPDATE OF subject, from_name, from_email, to_list, cc_list, snippet, body_text ON messages
			BEGIN
				INSERT INTO messages_fts(messages_fts, rowid, subject, from_name, from_email, to_list, cc_list, snippet, body_text)
				VALUES ('delete', OLD.rowid, OLD.subject, OLD.from_name, OLD.from_email, OLD.to_list, OLD.cc_list, OLD.snippet, OLD.body_text);
				INSERT INTO messages_fts(rowid, subject, from_name, from_email, to_list, cc_list, snippet, body_text)
				VALUES (NEW.rowid, NEW.subject, NEW.from_name, NEW.from_email, NEW.to_list, NEW.cc_list, NEW.snippet, NEW.body_text);
			END;
		`,
	},
	{
		Version: 47,
		SQL: `
			-- Remove legacy remote-contact, OAuth, and extension-secret schema.
			-- Current Contacts is local-only; remote records and token metadata are
			-- intentionally discarded.
			DELETE FROM contact_records WHERE source <> 'local';
			UPDATE contact_records SET source = 'local';

			DROP INDEX IF EXISTS idx_contact_records_source_ref;
			DROP TABLE IF EXISTS carddav_record_state;
			DROP TABLE IF EXISTS contact_source_oauth;
			DROP TABLE IF EXISTS contact_source_addressbooks;
			DROP TABLE IF EXISTS contact_sources;
			DROP TABLE IF EXISTS oauth_tokens;
			DROP TABLE IF EXISTS extension_secrets;
			DROP TABLE IF EXISTS user_oauth_clients;
			DROP TABLE IF EXISTS user_oauth_slot_aliases;
			DELETE FROM settings WHERE key LIKE 'oauth_active_choice:%';

			ALTER TABLE accounts DROP COLUMN encrypted_access_token;
			ALTER TABLE accounts DROP COLUMN encrypted_refresh_token;
			ALTER TABLE contact_records DROP COLUMN source_ref;
			ALTER TABLE contact_records DROP COLUMN vcard_raw;
		`,
	},
	{
		Version: 48,
		SQL: `
			-- Durable history for synchronization, backup, and future background
			-- activities. Type-specific fields live in payload_json so adding a new
			-- activity type does not require another schema change.
			CREATE TABLE activity_logs (
				id TEXT PRIMARY KEY,
				created_at TEXT NOT NULL,
				type TEXT NOT NULL,
				status TEXT NOT NULL,
				title TEXT NOT NULL,
				summary TEXT NOT NULL,
				detail TEXT,
				payload_json TEXT NOT NULL DEFAULT '{}'
			);

			CREATE INDEX idx_activity_logs_created_at
			ON activity_logs(created_at DESC);

			CREATE INDEX idx_activity_logs_type_created_at
			ON activity_logs(type, created_at DESC);

			CREATE INDEX idx_activity_logs_status_created_at
			ON activity_logs(status, created_at DESC);
		`,
	},
	{
		Version: 49,
		SQL: `
			-- Persist whether an IMAP LIST entry can be SELECTed. Hierarchy-only
			-- containers carry \Noselect and must remain visible in the folder tree
			-- without exposing stale local message rows as a real mailbox.
			-- This migration is intentionally additive and preserves every message.
			ALTER TABLE folders ADD COLUMN selectable INTEGER NOT NULL DEFAULT 1;
		`,
	},
}
