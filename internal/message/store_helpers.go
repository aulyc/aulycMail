package message

import (
	"fmt"
	"time"
)

// filterHavingClause returns a HAVING clause for conversation-level filtering.
// prefix should be "" for single-table queries or "m." for joined queries.
func filterHavingClause(filter, prefix string) string {
	switch filter {
	case "unread":
		return fmt.Sprintf(" HAVING SUM(CASE WHEN %sis_read = 0 THEN 1 ELSE 0 END) > 0", prefix)
	case "starred":
		return fmt.Sprintf(" HAVING MAX(CASE WHEN %sis_starred = 1 THEN 1 ELSE 0 END) = 1", prefix)
	case "attachments":
		return fmt.Sprintf(" HAVING MAX(CASE WHEN %shas_attachments = 1 THEN 1 ELSE 0 END) = 1", prefix)
	default:
		return ""
	}
}

// filterWhereClause returns a WHERE condition for count queries.
// prefix should be "" for single-table queries or "m." for joined queries.
func filterWhereClause(filter, prefix string) string {
	switch filter {
	case "unread":
		return fmt.Sprintf(" AND %sis_read = 0", prefix)
	case "starred":
		return fmt.Sprintf(" AND %sis_starred = 1", prefix)
	case "attachments":
		return fmt.Sprintf(" AND %shas_attachments = 1", prefix)
	default:
		return ""
	}
}

func applyComposeStatusRank(c *Conversation, rank int) {
	switch {
	case rank >= 20:
		c.ComposeStatus = ComposeStatusSent
	case rank >= 10:
		c.ComposeStatus = ComposeStatusDraft
	default:
		return
	}

	switch rank % 10 {
	case 3:
		c.ComposeAction = ComposeActionReplyAll
	case 2:
		c.ComposeAction = ComposeActionReply
	case 1:
		c.ComposeAction = ComposeActionForward
	}
}

// helper to convert empty string to NULL
func nullString(s string) interface{} {
	if s == "" {
		return nil
	}
	return s
}

// parseTimeString parses a time string in various formats
func parseTimeString(s string) time.Time {
	formats := []string{
		time.RFC3339,
		time.RFC3339Nano,
		"2006-01-02 15:04:05 -0700 MST", // Format used by Go's time.Time.String() when stored in SQLite
		"2006-01-02 15:04:05-07:00",
		"2006-01-02 15:04:05Z07:00",
		"2006-01-02 15:04:05",
		"2006-01-02T15:04:05",
		"2006-01-02",
	}
	for _, format := range formats {
		if parsed, err := time.Parse(format, s); err == nil {
			return parsed
		}
	}
	return time.Time{}
}
