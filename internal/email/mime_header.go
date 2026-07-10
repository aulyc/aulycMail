package email

import (
	"fmt"
	"io"
	"mime"
	"net/mail"
	"strings"

	msgcharset "github.com/emersion/go-message/charset"
	"golang.org/x/text/encoding/htmlindex"
)

// MIMECharsetReader returns a decoder for charsets seen in MIME encoded words.
func MIMECharsetReader(charsetName string, r io.Reader) (io.Reader, error) {
	if reader, err := msgcharset.Reader(charsetName, r); err == nil {
		return reader, nil
	}
	enc, err := htmlindex.Get(charsetName)
	if err != nil {
		return nil, fmt.Errorf("unknown charset: %s", charsetName)
	}
	return enc.NewDecoder().Reader(r), nil
}

// DecodeMIMEHeader decodes RFC 2047 encoded words with the same charset
// fallback used by attachment filename extraction.
func DecodeMIMEHeader(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	decoder := &mime.WordDecoder{CharsetReader: MIMECharsetReader}
	decoded, err := decoder.DecodeHeader(value)
	if err != nil {
		return value
	}
	return decoded
}

func ParseAddressHeader(value string) []string {
	value = DecodeMIMEHeader(value)
	if strings.TrimSpace(value) == "" {
		return nil
	}
	addresses, err := mail.ParseAddressList(value)
	if err != nil {
		return []string{value}
	}
	out := make([]string, 0, len(addresses))
	for _, address := range addresses {
		name := strings.TrimSpace(address.Name)
		if name == "" {
			out = append(out, address.Address)
			continue
		}
		out = append(out, fmt.Sprintf("%s <%s>", name, address.Address))
	}
	return out
}
