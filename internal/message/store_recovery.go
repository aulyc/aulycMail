package message

import "fmt"

// FindUniqueSelectableEquivalent finds a single real-mailbox row whose full
// locally cached envelope matches a retained row from a hierarchy-only folder.
// It deliberately returns nil for zero or multiple matches so recovery can
// never guess between messages that merely share a subject or date.
func (s *Store) FindUniqueSelectableEquivalent(messageID string) (*Message, error) {
	rows, err := s.db.Query(`
		SELECT candidate.id, candidate.account_id, candidate.folder_id,
		       candidate.uid, COALESCE(candidate.message_id, '')
		FROM messages target
		INNER JOIN messages candidate
			ON candidate.account_id = target.account_id
			AND candidate.id <> target.id
			AND COALESCE(candidate.subject, '') = COALESCE(target.subject, '')
			AND COALESCE(candidate.from_name, '') = COALESCE(target.from_name, '')
			AND COALESCE(candidate.from_email, '') = COALESCE(target.from_email, '')
			AND COALESCE(candidate.to_list, '') = COALESCE(target.to_list, '')
			AND COALESCE(candidate.cc_list, '') = COALESCE(target.cc_list, '')
			AND COALESCE(candidate.bcc_list, '') = COALESCE(target.bcc_list, '')
			AND COALESCE(candidate.reply_to, '') = COALESCE(target.reply_to, '')
			AND COALESCE(candidate.date, '') = COALESCE(target.date, '')
			AND COALESCE(candidate.size, 0) = COALESCE(target.size, 0)
			AND (
				TRIM(COALESCE(target.message_id, '')) = '' OR
				LOWER(TRIM(COALESCE(candidate.message_id, ''), '<> ')) =
				LOWER(TRIM(COALESCE(target.message_id, ''), '<> '))
			)
		INNER JOIN folders candidate_folder
			ON candidate_folder.id = candidate.folder_id
			AND candidate_folder.selectable = 1
		WHERE target.id = ?
		ORDER BY candidate.id
		LIMIT 2
	`, messageID)
	if err != nil {
		return nil, fmt.Errorf("failed to query selectable message equivalent: %w", err)
	}
	defer rows.Close()

	candidates := make([]*Message, 0, 2)
	for rows.Next() {
		candidate := &Message{}
		var uid int64
		if err := rows.Scan(
			&candidate.ID,
			&candidate.AccountID,
			&candidate.FolderID,
			&uid,
			&candidate.MessageID,
		); err != nil {
			return nil, fmt.Errorf("failed to scan selectable message equivalent: %w", err)
		}
		candidate.UID = uint32(uid)
		candidates = append(candidates, candidate)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to iterate selectable message equivalents: %w", err)
	}
	if len(candidates) != 1 {
		return nil, nil
	}
	return candidates[0], nil
}
