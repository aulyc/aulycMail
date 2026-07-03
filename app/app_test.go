package app

import (
	"strings"
	"testing"
)

func TestParseMailtoURL_Valid(t *testing.T) {
	result := ParseMailtoURL("mailto:test@example.com")
	if result == nil {
		t.Fatal("ParseMailtoURL returned nil, want non-nil")
	}
	if len(result.To) != 1 || result.To[0] != "test@example.com" {
		t.Errorf("To = %v, want [test@example.com]", result.To)
	}
}

func TestParseMailtoURL_WithParams(t *testing.T) {
	result := ParseMailtoURL("mailto:test@example.com?subject=Hello&body=World")
	if result == nil {
		t.Fatal("ParseMailtoURL returned nil, want non-nil")
	}
	if result.Subject != "Hello" {
		t.Errorf("Subject = %q, want %q", result.Subject, "Hello")
	}
	if result.Body != "World" {
		t.Errorf("Body = %q, want %q", result.Body, "World")
	}
}

func TestParseMailtoURL_MultiRecipient(t *testing.T) {
	result := ParseMailtoURL("mailto:a@b.com,c@d.com")
	if result == nil {
		t.Fatal("ParseMailtoURL returned nil, want non-nil")
	}
	if len(result.To) != 2 {
		t.Fatalf("len(To) = %d, want 2", len(result.To))
	}
	if result.To[0] != "a@b.com" {
		t.Errorf("To[0] = %q, want %q", result.To[0], "a@b.com")
	}
	if result.To[1] != "c@d.com" {
		t.Errorf("To[1] = %q, want %q", result.To[1], "c@d.com")
	}
}

func TestParseMailtoURL_WithCcBcc(t *testing.T) {
	result := ParseMailtoURL("mailto:a@b.com?cc=c@d.com&bcc=e@f.com")
	if result == nil {
		t.Fatal("ParseMailtoURL returned nil, want non-nil")
	}
	if len(result.Cc) != 1 || result.Cc[0] != "c@d.com" {
		t.Errorf("Cc = %v, want [c@d.com]", result.Cc)
	}
	if len(result.Bcc) != 1 || result.Bcc[0] != "e@f.com" {
		t.Errorf("Bcc = %v, want [e@f.com]", result.Bcc)
	}
}

func TestParseMailtoURL_TooLong(t *testing.T) {
	longURL := "mailto:test@example.com?" + strings.Repeat("x", 3000)
	result := ParseMailtoURL(longURL)
	if result != nil {
		t.Error("ParseMailtoURL with 3000+ char URL should return nil")
	}
}

func TestParseMailtoURL_InvalidScheme(t *testing.T) {
	result := ParseMailtoURL("http://example.com")
	if result != nil {
		t.Error("ParseMailtoURL with http:// scheme should return nil")
	}
}

func TestParseMailtoURL_Empty(t *testing.T) {
	result := ParseMailtoURL("")
	if result != nil {
		t.Error("ParseMailtoURL with empty string should return nil")
	}
}

func TestParseMailtoURL_NoAddress(t *testing.T) {
	result := ParseMailtoURL("mailto:?subject=Hello")
	if result == nil {
		t.Fatal("ParseMailtoURL returned nil, want non-nil")
	}
	if len(result.To) != 0 {
		t.Errorf("To = %v, want empty", result.To)
	}
	if result.Subject != "Hello" {
		t.Errorf("Subject = %q, want %q", result.Subject, "Hello")
	}
}

func TestRedactURLForLogRemovesSensitiveParts(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"https query", "https://example.com/reset?token=secret#frag", "https://example.com/reset"},
		{"userinfo", "https://user:pass@example.com/path?x=1", "https://example.com/path"},
		{"mailto query", "mailto:user@example.com?subject=secret&body=token", "mailto:user@example.com"},
		{"disallowed scheme", "data:text/html,secret-token", "data:[redacted]"},
		{"relative", "/reset?token=secret", "[relative-or-invalid-url length=19]"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := redactURLForLog(tt.in); got != tt.want {
				t.Fatalf("redactURLForLog(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestIsAllowedProtocolParsesScheme(t *testing.T) {
	tests := []struct {
		in   string
		want bool
	}{
		{"https://example.com/path?token=secret", true},
		{"HTTPS://example.com/path", true},
		{"http:example.com/path", false},
		{"mailto:user@example.com?subject=Hi", true},
		{"file:///tmp/secret", false},
		{"data:text/html,secret", false},
		{"/relative/path", false},
	}

	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			if got := isAllowedProtocol(tt.in); got != tt.want {
				t.Fatalf("isAllowedProtocol(%q) = %v, want %v", tt.in, got, tt.want)
			}
		})
	}
}
