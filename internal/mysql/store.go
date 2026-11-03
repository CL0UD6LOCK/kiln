package mysql

import (
	"context"
	"database/sql"

	"github.com/CL0UD6LOCK/kiln/internal/base"
)

type Store struct {
	db *sql.DB
}

func NewStore(db *sql.DB) *Store {
	return &Store{db: db}
}

func (s *Store) Enqueue(ctx context.Context, msg *base.TaskMessage) error {

}
