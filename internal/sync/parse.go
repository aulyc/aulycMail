package sync

import (
	"bytes"
	"context"
	"errors"
	"io"
	"mime"
	"strings"
	"time"

	"aulyc.local/aulycmail/internal/message"
	gomessage "github.com/emersion/go-message"
)

// parseMessageBodyFull parses a raw email and extracts text, HTML, and attachment metadata.
// Attachments are extracted during the same parsing pass - no re-parsing needed.
// For inline images, content is also captured (up to maxInlineContentSize) for display.
// For file attachments, only metadata is captured - content fetched on-demand.
// messageID is needed to create attachment records.
func (e *Engine) parseMessageBodyFull(raw []byte, messageID string, timeout time.Duration) *ParsedBody {
	ctx := context.Background()
	if timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, timeout)
		defer cancel()
	}

	parsed, err := e.parseMessageBodyInternal(ctx, raw, messageID)
	if err != nil {
		if isContextDone(err) {
			e.log.Warn().
				Int("rawLen", len(raw)).
				Dur("timeout", timeout).
				Msg("Body parsing timed out - attempting fallback extraction")
		} else {
			e.log.Debug().Err(err).Int("rawLen", len(raw)).Msg("Failed to parse message, trying fallback extraction")
		}
		e.log.Warn().
			Err(err).
			Int("rawLen", len(raw)).
			Msg("Body parser returned fallback content")
		partialText := e.extractPlainTextFallback(raw)
		return &ParsedBody{
			BodyText:       partialText,
			BodyHTML:       "",
			HasAttachments: false,
			Attachments:    nil,
		}
	}
	return parsed
}

// parseMessageBodyInternal does the actual parsing work, extracting body text, HTML, and attachments.
func (e *Engine) parseMessageBodyInternal(ctx context.Context, raw []byte, messageID string) (*ParsedBody, error) {
	result := &ParsedBody{}
	reader := newContextReader(ctx, bytes.NewReader(raw))

	entity, err := gomessage.Read(reader)
	if err != nil {
		if isContextDone(err) {
			return nil, err
		}
		// go-message can reject an unknown transfer encoding while reading the
		// top-level entity, before parseSinglePartBody gets a chance to inspect
		// the header. Do not fall back to exposing the raw message as body text.
		if gomessage.IsUnknownEncoding(err) {
			result.UnsafeContent = true
			result.BodyText = "This message uses non-standard encoding and cannot be displayed safely."
			e.log.Warn().Err(err).Int("rawLen", len(raw)).Msg("Unknown top-level encoding, marking unsafe")
			return result, nil
		}
		e.log.Debug().Err(err).Int("rawLen", len(raw)).Msg("Failed to parse message, trying as plain text")
		result.BodyText = string(raw)
		return result, nil
	}

	topLevelCT := entity.Header.Get("Content-Type")
	e.log.Debug().
		Str("topLevelContentType", topLevelCT).
		Int("rawLen", len(raw)).
		Msg("Parsing message body")

	mr := entity.MultipartReader()
	e.log.Debug().Bool("isMultipart", mr != nil).Msg("Multipart detection result")

	if mr != nil {
		if err := e.parseMultipartBody(ctx, mr, result, messageID); err != nil {
			return nil, err
		}
	}
	if mr == nil {
		if err := e.parseSinglePartBody(ctx, entity, result, messageID); err != nil {
			return nil, err
		}
	}

	e.log.Debug().
		Int("bodyTextLen", len(result.BodyText)).
		Int("bodyHTMLLen", len(result.BodyHTML)).
		Bool("hasAttachments", result.HasAttachments).
		Int("attachmentCount", len(result.Attachments)).
		Msg("parseMessageBody complete")

	return result, nil
}

// parseMultipartBody parses a multipart message body
func (e *Engine) parseMultipartBody(ctx context.Context, mr gomessage.MultipartReader, result *ParsedBody, messageID string) error {
	partIndex := 0
	for {
		if err := ctx.Err(); err != nil {
			return err
		}

		part, err := mr.NextPart()
		if err != nil {
			if isContextDone(err) {
				return err
			}
			if errors.Is(err, io.EOF) || strings.Contains(err.Error(), "EOF") {
				e.log.Debug().Int("partsProcessed", partIndex).Msg("Finished reading multipart parts")
				break
			}
			// go-message returns both the part AND an error for unknown CTE.
			// Mark as unsafe and stop — don't render content with invalid encoding.
			if gomessage.IsUnknownEncoding(err) {
				e.log.Warn().Err(err).Int("partsProcessed", partIndex).Msg("Unknown encoding in multipart part, marking unsafe")
				result.UnsafeContent = true
				result.BodyText = "This message uses non-standard encoding and cannot be displayed safely."
				return nil
			}
			e.log.Debug().Err(err).Int("partsProcessed", partIndex).Msg("Error reading multipart")
			break
		}
		partIndex++

		contentType, params, _ := mime.ParseMediaType(part.Header.Get("Content-Type"))
		disposition, dispParams, _ := mime.ParseMediaType(part.Header.Get("Content-Disposition"))
		contentID := strings.Trim(part.Header.Get("Content-ID"), "<>")

		e.log.Debug().
			Int("partIndex", partIndex).
			Str("contentType", contentType).
			Str("disposition", disposition).
			Str("charset", params["charset"]).
			Msg("Processing multipart part")

		// Handle file attachments
		if disposition == "attachment" {
			result.HasAttachments = true
			// If the attachment has a Content-ID, it's meant to be displayed inline in the HTML
			// (referenced via cid:contentID), even if Content-Disposition says "attachment"
			isInline := contentID != ""
			att, err := e.extractAttachmentMetadata(ctx, part, messageID, contentType, dispParams, contentID, isInline)
			if err != nil {
				return err
			}
			if att != nil {
				result.Attachments = append(result.Attachments, att)
			}
			continue
		}

		// Handle nested multipart
		if strings.HasPrefix(contentType, "multipart/") {
			if nestedMr := part.MultipartReader(); nestedMr != nil {
				if err := e.parseMultipartBody(ctx, nestedMr, result, messageID); err != nil {
					return err
				}
			}
			continue
		}

		// Handle inline images (explicit inline disposition OR image with Content-ID)
		// Many emails have images with Content-ID but no Content-Disposition header
		if (disposition == "inline" && strings.HasPrefix(contentType, "image/")) ||
			(contentID != "" && strings.HasPrefix(contentType, "image/")) {
			result.HasAttachments = true
			att, err := e.extractAttachmentMetadata(ctx, part, messageID, contentType, dispParams, contentID, true)
			if err != nil {
				return err
			}
			if att != nil {
				result.Attachments = append(result.Attachments, att)
			}
			continue
		}

		// Handle non-text, non-image parts with inline disposition (e.g. inline PDF)
		// or no disposition at all — these are implicit attachments
		if contentType != "" && !strings.HasPrefix(contentType, "text/") &&
			!isSignaturePart(contentType) {
			result.HasAttachments = true
			isInline := disposition == "inline" || contentID != ""
			att, err := e.extractAttachmentMetadata(ctx, part, messageID, contentType, dispParams, contentID, isInline)
			if err != nil {
				return err
			}
			if att != nil {
				result.Attachments = append(result.Attachments, att)
			}
			continue
		}

		// Check for invalid Content-Transfer-Encoding before reading.
		// go-message applies transfer decoding via part.Body, which fails on unknown encodings.
		// Invalid CTE is a common spammer technique — refuse to render for safety.
		cte := strings.ToLower(strings.TrimSpace(part.Header.Get("Content-Transfer-Encoding")))
		if cte != "" && cte != "7bit" && cte != "8bit" && cte != "binary" &&
			cte != "quoted-printable" && cte != "base64" {
			e.log.Warn().Str("cte", cte).Int("partIndex", partIndex).Msg("Non-standard Content-Transfer-Encoding, marking unsafe")
			result.UnsafeContent = true
			result.BodyText = "This message uses non-standard encoding and cannot be displayed safely."
			result.BodyHTML = ""
			return nil
		}

		// Read text parts
		partBody, err := readLimitedPart(ctx, part.Body, maxPartSize)
		if err != nil {
			if isContextDone(err) {
				return err
			}
			if len(partBody) > 0 {
				e.log.Warn().Err(err).Int("partIndex", partIndex).Int("partialLen", len(partBody)).Msg("Read partial part body")
			} else {
				e.log.Debug().Err(err).Int("partIndex", partIndex).Msg("Failed to read part body")
				continue
			}
		}

		if int64(len(partBody)) == maxPartSize {
			e.log.Warn().Int("partIndex", partIndex).Int64("maxSize", maxPartSize).Msg("Part body truncated")
		}

		e.log.Debug().Int("partIndex", partIndex).Int("partBodyLen", len(partBody)).Msg("Read part body successfully")

		charset := params["charset"]
		if charset == "" && contentType == "text/html" {
			charset = extractCharsetFromHTML(partBody)
		}
		decodedContent := decodeCharset(partBody, charset)

		switch contentType {
		case "text/plain":
			if result.BodyText == "" {
				result.BodyText = decodedContent
			}
		case "text/html":
			if result.BodyHTML == "" {
				result.BodyHTML = decodedContent
			}
		}
	}
	return nil
}

// parseSinglePartBody parses a single-part message body
func (e *Engine) parseSinglePartBody(ctx context.Context, entity *gomessage.Entity, result *ParsedBody, messageID string) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	contentType, params, _ := mime.ParseMediaType(entity.Header.Get("Content-Type"))
	e.log.Debug().Str("contentType", contentType).Str("charset", params["charset"]).Msg("Processing single-part message")

	// Check for invalid Content-Transfer-Encoding
	cte := strings.ToLower(strings.TrimSpace(entity.Header.Get("Content-Transfer-Encoding")))
	if cte != "" && cte != "7bit" && cte != "8bit" && cte != "binary" &&
		cte != "quoted-printable" && cte != "base64" {
		e.log.Warn().Str("cte", cte).Msg("Non-standard Content-Transfer-Encoding in single-part, marking unsafe")
		result.UnsafeContent = true
		result.BodyText = "This message uses non-standard encoding and cannot be displayed safely."
		return nil
	}

	// A single-part body can itself be an attachment (e.g. a DMARC report whose whole
	// body is application/zip). Mirror parseMultipartBody's classification so it becomes a
	// downloadable attachment instead of raw text dumped into the body.
	disposition, dispParams, _ := mime.ParseMediaType(entity.Header.Get("Content-Disposition"))
	contentID := strings.Trim(entity.Header.Get("Content-ID"), "<>")
	isNonTextBody := contentType != "" && !strings.HasPrefix(contentType, "text/") &&
		!isSignaturePart(contentType)
	if disposition == "attachment" || isNonTextBody {
		result.HasAttachments = true
		isInline := disposition == "inline" || contentID != ""
		att, err := e.extractAttachmentMetadata(ctx, entity, messageID, contentType, dispParams, contentID, isInline)
		if err != nil {
			return err
		}
		if att != nil {
			result.Attachments = append(result.Attachments, att)
		}
		return nil
	}

	body, err := readLimitedPart(ctx, entity.Body, maxPartSize)
	if err != nil {
		if isContextDone(err) {
			return err
		}
		e.log.Debug().Err(err).Msg("Failed to read single-part message body")
		return nil
	}

	if int64(len(body)) == maxPartSize {
		e.log.Warn().Int64("maxSize", maxPartSize).Msg("Single-part body truncated")
	}

	e.log.Debug().Int("bodyLen", len(body)).Msg("Read single-part message body")

	charset := params["charset"]
	if charset == "" && contentType == "text/html" {
		charset = extractCharsetFromHTML(body)
	}
	decodedContent := decodeCharset(body, charset)

	e.log.Debug().Int("decodedLen", len(decodedContent)).Msg("Decoded single-part content")

	switch contentType {
	case "text/html":
		result.BodyHTML = decodedContent
	default:
		result.BodyText = decodedContent
	}
	return nil
}

// parseMessageBody parses a raw email message and extracts text/plain and text/html parts.
// This is the legacy parsing path used by buildMessageFromStreamedData (via FetchServerMessage).
// The streamed parser path is preferred for new fetches.
func (e *Engine) parseMessageBody(raw []byte) (bodyText, bodyHTML string, hasAttachments bool) {
	reader := bytes.NewReader(raw)

	// Parse the message using go-message
	entity, err := gomessage.Read(reader)
	if err != nil {
		e.log.Debug().Err(err).Int("rawLen", len(raw)).Msg("Failed to parse message, trying as plain text")
		// If parsing fails, treat entire content as plain text
		return string(raw), "", false
	}

	// Log top-level Content-Type for debugging
	topLevelCT := entity.Header.Get("Content-Type")
	e.log.Debug().
		Str("topLevelContentType", topLevelCT).
		Int("rawLen", len(raw)).
		Msg("Parsing message body")

	// Check if it's a multipart message
	mr := entity.MultipartReader()
	e.log.Debug().Bool("isMultipart", mr != nil).Msg("Multipart detection result")

	if mr != nil {
		// Multipart message - iterate through parts
		partIndex := 0
		for {
			part, err := mr.NextPart()
			if err != nil {
				// EOF (or wrapped EOF like "multipart: NextPart: EOF") signals end of parts
				if !errors.Is(err, io.EOF) && !strings.Contains(err.Error(), "EOF") {
					e.log.Debug().Err(err).Int("partsProcessed", partIndex).Msg("Error reading multipart")
				} else {
					e.log.Debug().Int("partsProcessed", partIndex).Msg("Finished reading multipart parts")
				}
				break
			}
			partIndex++

			contentType, params, _ := mime.ParseMediaType(part.Header.Get("Content-Type"))
			disposition, _, _ := mime.ParseMediaType(part.Header.Get("Content-Disposition"))

			e.log.Debug().
				Int("partIndex", partIndex).
				Str("contentType", contentType).
				Str("disposition", disposition).
				Str("charset", params["charset"]).
				Msg("Processing multipart part")

			// Check for attachments
			if disposition == "attachment" {
				hasAttachments = true
				continue
			}

			// Handle nested multipart
			if strings.HasPrefix(contentType, "multipart/") {
				nestedText, nestedHTML, nestedAttach := e.parseNestedMultipart(part)
				if bodyText == "" {
					bodyText = nestedText
				}
				if bodyHTML == "" {
					bodyHTML = nestedHTML
				}
				hasAttachments = hasAttachments || nestedAttach
				continue
			}

			// Read the part body with size limit to prevent memory exhaustion
			lr := io.LimitReader(part.Body, maxPartSize)
			partBody, err := io.ReadAll(lr)
			if err != nil {
				// Check if we got partial data despite the error (e.g., malformed email missing closing boundary)
				if len(partBody) > 0 {
					e.log.Warn().
						Err(err).
						Int("partIndex", partIndex).
						Int("partialLen", len(partBody)).
						Msg("Read partial part body despite error, using partial data")
					// Continue processing with partial data (don't skip)
				} else {
					e.log.Debug().Err(err).Int("partIndex", partIndex).Msg("Failed to read part body, no data recovered")
					continue
				}
			}

			// Log if we hit the size limit (truncated)
			if int64(len(partBody)) == maxPartSize {
				e.log.Warn().
					Int("partIndex", partIndex).
					Int64("maxSize", maxPartSize).
					Msg("Part body truncated at size limit - saving partial content")
			}

			e.log.Debug().
				Int("partIndex", partIndex).
				Int("partBodyLen", len(partBody)).
				Msg("Read part body successfully")

			// First, check if content needs explicit quoted-printable decoding
			// (go-message should handle this via Entity.Body, but some edge cases might slip through)
			partBody = decodeQuotedPrintableIfNeeded(partBody)

			// Decode charset to UTF-8
			charset := params["charset"]

			// If no charset in header and this is HTML, try to extract from meta tags
			if charset == "" && contentType == "text/html" {
				charset = extractCharsetFromHTML(partBody)
				e.log.Debug().
					Str("charsetFromHTML", charset).
					Msg("Extracted charset from HTML meta tags")
			}
			decodedContent := decodeCharset(partBody, charset)

			// Debug: Check if content still contains quoted-printable sequences
			if contentType == "text/html" && len(decodedContent) > 200 {
				snippet := decodedContent
				if len(snippet) > 200 {
					snippet = snippet[:200]
				}
				e.log.Debug().
					Str("htmlSnippet", snippet).
					Bool("hasQuotedPrintable", strings.Contains(decodedContent, "=3D")).
					Msg("HTML content analysis")
			}

			switch contentType {
			case "text/plain":
				if bodyText == "" {
					bodyText = decodedContent
				}
			case "text/html":
				if bodyHTML == "" {
					bodyHTML = decodedContent
				}
			default:
				// Other content types might be inline attachments
				if disposition == "inline" && strings.HasPrefix(contentType, "image/") {
					// Inline images need to be extracted so they can be displayed
					hasAttachments = true
				} else if contentType != "" && !strings.HasPrefix(contentType, "text/") {
					hasAttachments = true
				}
			}
		}
	} else {
		// Single part message
		contentType, params, _ := mime.ParseMediaType(entity.Header.Get("Content-Type"))
		e.log.Debug().
			Str("contentType", contentType).
			Str("charset", params["charset"]).
			Msg("Processing single-part message")

		// Read with size limit to prevent memory exhaustion
		lr := io.LimitReader(entity.Body, maxPartSize)
		body, err := io.ReadAll(lr)
		if err != nil {
			e.log.Debug().Err(err).Msg("Failed to read single-part message body")
			return "", "", false
		}

		// Log if we hit the size limit (truncated)
		if int64(len(body)) == maxPartSize {
			e.log.Warn().
				Int64("maxSize", maxPartSize).
				Msg("Single-part body truncated at size limit - saving partial content")
		}

		e.log.Debug().Int("bodyLen", len(body)).Msg("Read single-part message body")

		// First, check if content needs explicit quoted-printable decoding
		body = decodeQuotedPrintableIfNeeded(body)

		// Decode charset to UTF-8
		charset := params["charset"]
		// If no charset in header and this is HTML, try to extract from meta tags
		if charset == "" && contentType == "text/html" {
			charset = extractCharsetFromHTML(body)
		}
		decodedContent := decodeCharset(body, charset)

		e.log.Debug().Int("decodedLen", len(decodedContent)).Msg("Decoded single-part content")

		switch contentType {
		case "text/html":
			bodyHTML = decodedContent
		default:
			// Default to plain text
			bodyText = decodedContent
		}
	}

	// Log final result
	e.log.Debug().
		Int("bodyTextLen", len(bodyText)).
		Int("bodyHTMLLen", len(bodyHTML)).
		Bool("hasAttachments", hasAttachments).
		Msg("parseMessageBody complete")

	return bodyText, bodyHTML, hasAttachments
}

// parseNestedMultipart handles nested multipart structures.
// This is part of the legacy parsing path used by parseMessageBody.
func (e *Engine) parseNestedMultipart(entity *gomessage.Entity) (bodyText, bodyHTML string, hasAttachments bool) {
	mr := entity.MultipartReader()
	if mr == nil {
		return "", "", false
	}

	for {
		part, err := mr.NextPart()
		if err != nil {
			// EOF (or wrapped EOF) signals end of parts - no need to log
			break
		}

		contentType, params, _ := mime.ParseMediaType(part.Header.Get("Content-Type"))
		disposition, _, _ := mime.ParseMediaType(part.Header.Get("Content-Disposition"))

		if disposition == "attachment" {
			hasAttachments = true
			continue
		}

		if strings.HasPrefix(contentType, "multipart/") {
			nestedText, nestedHTML, nestedAttach := e.parseNestedMultipart(part)
			if bodyText == "" {
				bodyText = nestedText
			}
			if bodyHTML == "" {
				bodyHTML = nestedHTML
			}
			hasAttachments = hasAttachments || nestedAttach
			continue
		}

		// Read with size limit to prevent memory exhaustion
		lr := io.LimitReader(part.Body, maxPartSize)
		partBody, err := io.ReadAll(lr)
		if err != nil {
			// Check if we got partial data despite the error (e.g., malformed email missing closing boundary)
			if len(partBody) > 0 {
				e.log.Warn().
					Err(err).
					Int("partialLen", len(partBody)).
					Msg("Read partial nested part body despite error, using partial data")
				// Continue processing with partial data (don't skip)
			} else {
				continue
			}
		}

		// Log if we hit the size limit (truncated)
		if int64(len(partBody)) == maxPartSize {
			e.log.Warn().
				Int64("maxSize", maxPartSize).
				Msg("Nested part body truncated at size limit - saving partial content")
		}

		// Decode charset to UTF-8
		charset := params["charset"]
		// If no charset in header and this is HTML, try to extract from meta tags
		if charset == "" && contentType == "text/html" {
			charset = extractCharsetFromHTML(partBody)
		}
		decodedContent := decodeCharset(partBody, charset)

		switch contentType {
		case "text/plain":
			if bodyText == "" {
				bodyText = decodedContent
			}
		case "text/html":
			if bodyHTML == "" {
				bodyHTML = decodedContent
			}
		}
	}

	return bodyText, bodyHTML, hasAttachments
}

// extractAttachmentMetadata extracts attachment metadata from a MIME part.
// For inline images, also captures content (up to maxInlineContentSize).
// For file attachments, reads content to get size but doesn't store it (fetched on-demand).
func (e *Engine) extractAttachmentMetadata(ctx context.Context, part *gomessage.Entity, messageID, contentType string, dispParams map[string]string, contentID string, isInline bool) (*message.Attachment, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	filename := dispParams["filename"]
	if filename == "" {
		// Try to get from Content-Type name parameter
		_, ctParams, _ := mime.ParseMediaType(part.Header.Get("Content-Type"))
		filename = ctParams["name"]
	}
	// Decode RFC 2047 encoded filenames (e.g., =?UTF-8?B?5Lit5paH?= for Chinese)
	filename = decodeMIMEWord(filename)
	if filename == "" {
		// Generate a filename based on content type
		ext := ".bin"
		if strings.HasPrefix(contentType, "image/") {
			parts := strings.Split(contentType, "/")
			if len(parts) == 2 {
				ext = "." + parts[1]
			}
		}
		filename = "attachment" + ext
	}

	att := &message.Attachment{
		ID:          generateID(),
		MessageID:   messageID,
		Filename:    filename,
		ContentType: contentType,
		ContentID:   contentID,
		IsInline:    isInline,
	}

	// Read the attachment content
	content, err := readLimitedPart(ctx, part.Body, maxPartSize)
	if err != nil {
		if isContextDone(err) {
			return nil, err
		}
		e.log.Debug().Err(err).Str("filename", filename).Msg("Failed to read attachment content")
		return att, nil
	}

	att.Size = len(content)

	if isInline {
		// For inline images, store content (needed for display in email)
		// But limit to maxInlineContentSize to prevent huge DB entries
		if len(content) <= maxInlineContentSize {
			att.Content = content
			e.log.Debug().Str("filename", filename).Int("size", len(content)).Msg("Extracted inline attachment with content")
		} else {
			e.log.Debug().Str("filename", filename).Int("size", len(content)).Msg("Inline attachment too large, stored metadata only")
		}
	} else {
		// For file attachments, we have the size but don't store content
		// Content will be fetched on-demand when user downloads
		e.log.Debug().Str("filename", filename).Int("size", len(content)).Msg("Extracted file attachment metadata")
	}

	return att, nil
}

type contextReader struct {
	ctx context.Context
	r   io.Reader
}

func newContextReader(ctx context.Context, r io.Reader) io.Reader {
	if ctx == nil {
		return r
	}
	return &contextReader{ctx: ctx, r: r}
}

func (r *contextReader) Read(p []byte) (int, error) {
	if err := r.ctx.Err(); err != nil {
		return 0, err
	}
	n, err := r.r.Read(p)
	if err != nil {
		return n, err
	}
	if ctxErr := r.ctx.Err(); ctxErr != nil {
		return n, ctxErr
	}
	return n, nil
}

func readLimitedPart(ctx context.Context, r io.Reader, maxBytes int64) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return io.ReadAll(io.LimitReader(newContextReader(ctx, r), maxBytes))
}

func isContextDone(err error) bool {
	return errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded)
}

// isSignaturePart returns true for S/MIME and PGP signature content types
// that should not be exposed as downloadable attachments.
func isSignaturePart(contentType string) bool {
	switch contentType {
	case "application/pkcs7-signature", "application/x-pkcs7-signature",
		"application/pgp-signature":
		return true
	}
	return false
}

// extractPlainTextFallback attempts to extract readable text from raw email bytes.
// Used when normal parsing times out or fails completely.
// Returns partial content which is better than nothing.
func (e *Engine) extractPlainTextFallback(raw []byte) string {
	// Find the body (after double CRLF or double LF - standard email header/body separator)
	bodyStart := bytes.Index(raw, []byte("\r\n\r\n"))
	bodyOffset := 4
	if bodyStart == -1 {
		bodyStart = bytes.Index(raw, []byte("\n\n"))
		bodyOffset = 2
	}
	if bodyStart == -1 {
		// No header/body separator found, can't extract safely
		return ""
	}

	body := raw[bodyStart+bodyOffset:]

	// Extract printable ASCII characters as a last resort
	// This handles cases where content might be partially encoded
	var result strings.Builder
	const maxFallbackSize = 10 * 1024
	result.Grow(min(len(body), maxFallbackSize))
	for _, b := range body {
		if (b >= 32 && b < 127) || b == '\n' || b == '\r' || b == '\t' {
			result.WriteByte(b)
			if result.Len() >= maxFallbackSize {
				break
			}
		}
	}

	text := strings.TrimSpace(result.String())

	// Limit to first 10KB to prevent huge partial extractions
	if len(text) >= maxFallbackSize {
		text += "... [truncated - parsing timed out]"
	}

	if text != "" {
		e.log.Info().
			Int("extractedLen", len(text)).
			Msg("Extracted partial text via fallback")
	}

	return text
}
