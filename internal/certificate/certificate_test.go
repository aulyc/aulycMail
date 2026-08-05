package certificate

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"database/sql"
	"errors"
	"math/big"
	"testing"
	"time"

	_ "modernc.org/sqlite"
)

func generateTestCert(t *testing.T) []byte {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("failed to generate key: %v", err)
	}
	template := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "test.example.com", Organization: []string{"Test Org"}},
		NotBefore:    time.Now().Add(-1 * time.Hour),
		NotAfter:     time.Now().Add(24 * time.Hour),
		DNSNames:     []string{"test.example.com"},
	}
	derBytes, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		t.Fatalf("failed to create certificate: %v", err)
	}
	return derBytes
}

func openTestStore(t *testing.T) *Store {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("failed to open in-memory DB: %v", err)
	}
	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS trusted_certificates (
		id TEXT PRIMARY KEY,
		fingerprint TEXT NOT NULL,
		host TEXT NOT NULL,
		subject TEXT,
		issuer TEXT,
		not_before TEXT,
		not_after TEXT,
		accepted_at DATETIME,
		UNIQUE(host, fingerprint)
	)`)
	if err != nil {
		t.Fatalf("failed to create table: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return NewStore(db)
}

func TestFingerprint(t *testing.T) {
	der := generateTestCert(t)
	fp := Fingerprint(der)

	if len(fp) != 64 {
		t.Fatalf("Fingerprint length = %d, want 64", len(fp))
	}

	// Verify it's hex
	for _, c := range fp {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')) {
			t.Fatalf("Fingerprint contains non-hex char: %c", c)
		}
	}
}

func TestExtractCertInfo(t *testing.T) {
	der := generateTestCert(t)
	info := ExtractCertInfo(der, errors.New("test error"))

	if info.Subject == "" {
		t.Fatal("Subject should not be empty")
	}
	if info.Issuer == "" {
		t.Fatal("Issuer should not be empty")
	}
	if len(info.DNSNames) == 0 {
		t.Fatal("DNSNames should not be empty")
	}
	if info.DNSNames[0] != "test.example.com" {
		t.Fatalf("DNSNames[0] = %q, want %q", info.DNSNames[0], "test.example.com")
	}
	if info.NotBefore == "" {
		t.Fatal("NotBefore should not be empty")
	}
	if info.NotAfter == "" {
		t.Fatal("NotAfter should not be empty")
	}
	if info.Fingerprint == "" {
		t.Fatal("Fingerprint should not be empty")
	}
	if info.IsExpired {
		t.Fatal("IsExpired = true, want false (cert valid for 24h)")
	}
}

func TestFormatDN(t *testing.T) {
	tests := []struct {
		name string
		cn   string
		org  []string
		want string
	}{
		{"cn and org", "example.com", []string{"Org"}, "example.com (Org)"},
		{"cn only", "example.com", nil, "example.com"},
		{"org only", "", []string{"Org"}, "Org"},
		{"neither", "", nil, "(unknown)"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatDN(tt.cn, tt.org)
			if got != tt.want {
				t.Fatalf("formatDN(%q, %v) = %q, want %q", tt.cn, tt.org, got, tt.want)
			}
		})
	}
}

func TestClassifyError(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want string
	}{
		{"nil error", nil, "unknown error"},
		{"unknown authority", errors.New("x509: certificate signed by unknown authority"), "self-signed or unknown certificate authority"},
		{"expired", errors.New("x509: certificate has expired or is not yet valid"), "certificate has expired"},
		{"random error", errors.New("something went wrong"), "something went wrong"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := classifyError(tt.err)
			if got != tt.want {
				t.Fatalf("classifyError(%v) = %q, want %q", tt.err, got, tt.want)
			}
		})
	}
}

func TestAcceptSession(t *testing.T) {
	store := openTestStore(t)

	fp := "aabbccdd11223344aabbccdd11223344aabbccdd11223344aabbccdd11223344"
	if err := store.AcceptSession("imap.example.com", fp); err != nil {
		t.Fatalf("AcceptSession() error = %v", err)
	}

	if !store.IsTrusted("imap.example.com", fp) {
		t.Fatal("IsTrusted = false after AcceptSession, want true")
	}
	if store.IsTrusted("smtp.example.com", fp) {
		t.Fatal("IsTrusted = true for different host, want false")
	}
}

func TestIsTrustedDefault(t *testing.T) {
	store := openTestStore(t)

	fp := "0000000000000000000000000000000000000000000000000000000000000000"
	if store.IsTrusted("imap.example.com", fp) {
		t.Fatal("IsTrusted = true for unknown fingerprint, want false")
	}
}

func TestAcceptPermanentlyScopesTrustToHost(t *testing.T) {
	store := openTestStore(t)
	der := generateTestCert(t)
	info := ExtractCertInfo(der, errors.New("self-signed"))

	if err := store.AcceptPermanently("IMAP.EXAMPLE.COM", info); err != nil {
		t.Fatalf("AcceptPermanently() error = %v", err)
	}
	if !store.IsTrusted("imap.example.com", info.Fingerprint) {
		t.Fatal("trusted certificate was not accepted for normalized host")
	}
	if store.IsTrusted("smtp.example.com", info.Fingerprint) {
		t.Fatal("trusted certificate leaked to a different host")
	}
}

func TestTrustedCertificateListingAndRemovalNormalizesInputs(t *testing.T) {
	store := openTestStore(t)
	now := time.Now().UTC()
	first := &CertificateInfo{
		Fingerprint: " AA11 ",
		Subject:     "first",
		Issuer:      "issuer",
		NotBefore:   now.Add(-time.Hour).Format(time.RFC3339),
		NotAfter:    now.Add(time.Hour).Format(time.RFC3339),
	}
	second := &CertificateInfo{
		Fingerprint: "BB22",
		Subject:     "second",
		Issuer:      "issuer",
		NotBefore:   now.Add(-time.Hour).Format(time.RFC3339),
		NotAfter:    now.Add(time.Hour).Format(time.RFC3339),
	}
	if err := store.AcceptPermanently(" IMAP.EXAMPLE.COM ", first); err != nil {
		t.Fatalf("AcceptPermanently(first) error = %v", err)
	}
	if err := store.AcceptPermanently("smtp.example.com", second); err != nil {
		t.Fatalf("AcceptPermanently(second) error = %v", err)
	}
	if err := store.AcceptSession("imap.example.com", "AA11"); err != nil {
		t.Fatalf("AcceptSession() error = %v", err)
	}

	certs, err := store.GetByHosts([]string{" IMAP.EXAMPLE.COM ", "imap.example.com", "", "missing.example.com"})
	if err != nil {
		t.Fatalf("GetByHosts() error = %v", err)
	}
	if len(certs) != 1 || certs[0].Host != "imap.example.com" || certs[0].Fingerprint != "aa11" || certs[0].Subject != "first" {
		t.Fatalf("GetByHosts() = %#v", certs)
	}
	if empty, err := store.GetByHosts(nil); err != nil || len(empty) != 0 {
		t.Fatalf("GetByHosts(nil) = %#v, %v", empty, err)
	}
	if empty, err := store.GetByHosts([]string{" ", ""}); err != nil || len(empty) != 0 {
		t.Fatalf("GetByHosts(blank) = %#v, %v", empty, err)
	}

	if err := store.Remove(" AA11 "); err != nil {
		t.Fatalf("Remove() error = %v", err)
	}
	if store.IsTrusted("imap.example.com", "aa11") {
		t.Fatal("certificate remained trusted after removing persistent and session entries")
	}
	remaining, err := store.GetByHosts([]string{"imap.example.com", "smtp.example.com"})
	if err != nil || len(remaining) != 1 || remaining[0].Fingerprint != "bb22" {
		t.Fatalf("remaining certificates = %#v, %v", remaining, err)
	}
}

func TestCertificateStoreRejectsIncompleteTrustInputs(t *testing.T) {
	store := openTestStore(t)
	valid := &CertificateInfo{Fingerprint: "aa11"}
	for _, test := range []struct {
		name string
		err  error
	}{
		{name: "permanent blank host", err: store.AcceptPermanently(" ", valid)},
		{name: "permanent nil info", err: store.AcceptPermanently("imap.example.com", nil)},
		{name: "permanent blank fingerprint", err: store.AcceptPermanently("imap.example.com", &CertificateInfo{})},
		{name: "session blank host", err: store.AcceptSession("", "aa11")},
		{name: "session blank fingerprint", err: store.AcceptSession("imap.example.com", " ")},
	} {
		if test.err == nil {
			t.Fatalf("%s error = nil, want validation error", test.name)
		}
	}
	if store.IsTrusted("", "aa11") || store.IsTrusted("imap.example.com", "") {
		t.Fatal("blank host or fingerprint was trusted")
	}
}

func TestBuildTLSConfigTrustedCertificateRequiresMatchingHost(t *testing.T) {
	store := openTestStore(t)
	der := generateTestCert(t)
	fp := Fingerprint(der)

	if err := store.AcceptSession("test.example.com", fp); err != nil {
		t.Fatalf("AcceptSession() error = %v", err)
	}

	if err := BuildTLSConfig("test.example.com", store).VerifyPeerCertificate([][]byte{der}, nil); err != nil {
		t.Fatalf("VerifyPeerCertificate() trusted host error = %v", err)
	}
	if err := BuildTLSConfig("other.example.com", store).VerifyPeerCertificate([][]byte{der}, nil); err == nil {
		t.Fatal("VerifyPeerCertificate() trusted certificate for different host, want error")
	}
}

func TestErrorInterface(t *testing.T) {
	info := &CertificateInfo{
		Fingerprint: "abcd1234",
	}
	certErr := &Error{
		Info:   info,
		Reason: "test reason",
	}

	// Verify it implements the error interface
	var err error = certErr
	if err.Error() == "" {
		t.Fatal("Error() should return non-empty string")
	}

	expected := "untrusted certificate: test reason (fingerprint: abcd1234)"
	if err.Error() != expected {
		t.Fatalf("Error() = %q, want %q", err.Error(), expected)
	}
}
