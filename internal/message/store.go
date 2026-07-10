package message

import (
	"github.com/aulyc/aulycmail/internal/database"
	"github.com/aulyc/aulycmail/internal/logging"
	"github.com/rs/zerolog"
)

// Store provides message persistence operations
type Store struct {
	db  *database.DB
	log zerolog.Logger
}

// NewStore creates a new message store
func NewStore(db *database.DB) *Store {
	return &Store{
		db:  db,
		log: logging.WithComponent("message-store"),
	}
}
