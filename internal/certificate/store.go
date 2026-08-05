package certificate

import (
	"database/sql"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

// Store manages trusted certificates in the database and session memory
type Store struct {
	db      *sql.DB
	mu      sync.RWMutex
	session map[string]bool // host + fingerprint -> trusted (session only)
}

// NewStore creates a new certificate trust store
func NewStore(db *sql.DB) *Store {
	return &Store{
		db:      db,
		session: make(map[string]bool),
	}
}

// IsTrusted checks if a certificate fingerprint is trusted for this host.
func (s *Store) IsTrusted(host, fingerprint string) bool {
	host = normalizeHost(host)
	fingerprint = normalizeFingerprint(fingerprint)
	if host == "" || fingerprint == "" {
		return false
	}

	// Check session memory first (fast path)
	key := trustKey(host, fingerprint)
	s.mu.RLock()
	if s.session[key] {
		s.mu.RUnlock()
		return true
	}
	s.mu.RUnlock()

	// Check database
	var count int
	err := s.db.QueryRow(
		"SELECT COUNT(*) FROM trusted_certificates WHERE host = ? AND fingerprint = ?",
		host, fingerprint,
	).Scan(&count)
	if err != nil {
		return false
	}
	return count > 0
}

// AcceptPermanently stores a certificate in the database
func (s *Store) AcceptPermanently(host string, info *CertificateInfo) error {
	host = normalizeHost(host)
	if host == "" {
		return fmt.Errorf("certificate host is required")
	}
	if info == nil {
		return fmt.Errorf("certificate info is required")
	}
	info.Fingerprint = normalizeFingerprint(info.Fingerprint)
	if info.Fingerprint == "" {
		return fmt.Errorf("certificate fingerprint is required")
	}

	id := uuid.New().String()
	_, err := s.db.Exec(
		`INSERT OR REPLACE INTO trusted_certificates (id, fingerprint, host, subject, issuer, not_before, not_after, accepted_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		id, info.Fingerprint, host, info.Subject, info.Issuer, info.NotBefore, info.NotAfter, time.Now(),
	)
	return err
}

// AcceptSession stores a host-scoped certificate fingerprint in session memory only.
func (s *Store) AcceptSession(host, fingerprint string) error {
	host = normalizeHost(host)
	fingerprint = normalizeFingerprint(fingerprint)
	if host == "" {
		return fmt.Errorf("certificate host is required")
	}
	if fingerprint == "" {
		return fmt.Errorf("certificate fingerprint is required")
	}

	s.mu.Lock()
	s.session[trustKey(host, fingerprint)] = true
	s.mu.Unlock()
	return nil
}

// GetByHosts returns permanently trusted certificates for the given hosts
func (s *Store) GetByHosts(hosts []string) ([]*CertificateInfo, error) {
	if len(hosts) == 0 {
		return nil, nil
	}

	// Build query with placeholders
	query := "SELECT fingerprint, host, subject, issuer, not_before, not_after FROM trusted_certificates WHERE host IN ("
	normalizedHosts := make([]string, 0, len(hosts))
	seen := make(map[string]bool, len(hosts))
	for _, h := range hosts {
		h = normalizeHost(h)
		if h == "" || seen[h] {
			continue
		}
		seen[h] = true
		normalizedHosts = append(normalizedHosts, h)
	}
	if len(normalizedHosts) == 0 {
		return nil, nil
	}

	args := make([]interface{}, len(normalizedHosts))
	for i, h := range normalizedHosts {
		if i > 0 {
			query += ","
		}
		query += "?"
		args[i] = h
	}
	query += ") ORDER BY accepted_at DESC"

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var certs []*CertificateInfo
	for rows.Next() {
		var ci CertificateInfo
		var host string
		if err := rows.Scan(&ci.Fingerprint, &host, &ci.Subject, &ci.Issuer, &ci.NotBefore, &ci.NotAfter); err != nil {
			return nil, err
		}
		ci.Host = host
		certs = append(certs, &ci)
	}
	return certs, rows.Err()
}

// Remove deletes a trusted certificate from the database by fingerprint
func (s *Store) Remove(fingerprint string) error {
	fingerprint = normalizeFingerprint(fingerprint)
	_, err := s.db.Exec("DELETE FROM trusted_certificates WHERE fingerprint = ?", fingerprint)
	if err != nil {
		return err
	}

	// Also remove from session
	s.mu.Lock()
	for key := range s.session {
		if strings.HasSuffix(key, "\x00"+fingerprint) {
			delete(s.session, key)
		}
	}
	s.mu.Unlock()

	return nil
}

func normalizeHost(host string) string {
	return strings.ToLower(strings.TrimSpace(host))
}

func normalizeFingerprint(fingerprint string) string {
	return strings.ToLower(strings.TrimSpace(fingerprint))
}

func trustKey(host, fingerprint string) string {
	return host + "\x00" + fingerprint
}
