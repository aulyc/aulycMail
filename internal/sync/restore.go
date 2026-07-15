package sync

import (
	"context"
	"errors"
	"fmt"
	"io"
	"time"

	"aulyc.local/aulycmail/internal/message"
)

// RestoreMessageBodyFromReader restores a message from a complete raw RFC822
// source after enforcing the same size, identity, parsing, and sanitization
// rules used by an on-demand IMAP body fetch.
func (e *Engine) RestoreMessageBodyFromReader(ctx context.Context, messageID string, raw io.Reader) (*message.Message, error) {
	if raw == nil {
		return nil, errors.New("raw message reader is nil")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	localMessage, err := e.messageStore.Get(messageID)
	if err != nil {
		return nil, fmt.Errorf("failed to get message info: %w", err)
	}
	if localMessage == nil {
		return nil, fmt.Errorf("message not found: %s", messageID)
	}

	rawBytes, err := readRawMessageLiteralWithLimit(
		localMessage.UID,
		newContextReader(ctx, raw),
		maxBackgroundRawBodyBytes,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to read backup message: %w", err)
	}
	if len(rawBytes) == 0 {
		return nil, errors.New("backup message is empty")
	}
	if err := validateFetchedMessageIdentity(localMessage.UID, localMessage.MessageID, rawBytes); err != nil {
		return nil, err
	}

	result := e.processRawMessageBody(rawBytes, localMessage.ID)
	if result.BodyHTML == "" && result.BodyText == "" && len(result.Attachments) == 0 {
		return nil, errors.New("backup message contains no usable body or attachments")
	}
	if err := e.messageStore.RestoreBody(
		localMessage.ID,
		result.BodyHTML,
		result.BodyText,
		result.Snippet,
		result.HasAttachments,
		result.Attachments,
	); err != nil {
		return nil, err
	}
	return e.messageStore.Get(localMessage.ID)
}

// ValidateMessageIdentityFromReader verifies that a raw source belongs to the
// requested local row without persisting or exposing its contents.
func (e *Engine) ValidateMessageIdentityFromReader(ctx context.Context, messageID string, raw io.Reader) error {
	if raw == nil {
		return errors.New("raw message reader is nil")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	localMessage, err := e.messageStore.Get(messageID)
	if err != nil {
		return fmt.Errorf("failed to get message info: %w", err)
	}
	if localMessage == nil {
		return fmt.Errorf("message not found: %s", messageID)
	}
	if normalizeRFCMessageID(localMessage.MessageID) == "" {
		return nil
	}
	return validateFetchedMessageIdentityFromReader(
		localMessage.UID,
		localMessage.MessageID,
		io.LimitReader(newContextReader(ctx, raw), maxHeaderLiteralBytes+1),
	)
}

func (e *Engine) processRawMessageBody(rawBytes []byte, messageID string) *ProcessedBody {
	parsed := e.parseMessageBodyFull(rawBytes, messageID, 30*time.Second)
	bodyHTML := parsed.BodyHTML
	if bodyHTML != "" {
		bodyHTML = e.sanitizer.Sanitize(bodyHTML)
	}

	var snippet string
	if parsed.BodyText != "" {
		snippet = generateSnippet(parsed.BodyText, 200)
	} else if bodyHTML != "" {
		snippet = generateSnippet(stripHTMLTags(bodyHTML), 200)
	}
	return &ProcessedBody{
		MessageID:      messageID,
		BodyHTML:       bodyHTML,
		BodyText:       parsed.BodyText,
		Snippet:        snippet,
		HasAttachments: parsed.HasAttachments,
		Attachments:    parsed.Attachments,
		RawBytes:       rawBytes,
		ReceivedBytes:  int64(len(rawBytes)),
	}
}
