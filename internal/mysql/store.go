package mysql

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"time"

	"github.com/CL0UD6LOCK/kiln/internal/base"
)

type Store struct {
	db *sql.DB
}

func NewStore(db *sql.DB) *Store {
	return &Store{db: db}
}

func (s *Store) Enqueue(ctx context.Context, msg *base.TaskMessage) error {
	if !json.Valid(msg.Payload) {
		return errors.New("kiln: payload is not valid JSON")
	}
	if msg.MaxAttempts <= 0 {
		msg.MaxAttempts = 3
	}
	if msg.RunAt.IsZero() {
		msg.RunAt = time.Now().UTC()
	}
	res, err := s.db.ExecContext(ctx, "INSERT INTO tasks (type,payload,max_attempts,run_at) VALUES(?,?,?,?)", msg.Type, string(msg.Payload), msg.MaxAttempts, msg.RunAt)
	if err != nil {
		return err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return err
	}
	msg.ID = id
	return nil
}
