package mysql

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
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

func (s *Store) Claim(ctx context.Context) (*base.TaskMessage, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	msg := &base.TaskMessage{}
	err = tx.QueryRowContext(ctx, "SELECT id,type,payload FROM tasks WHERE status ='pending' AND run_at<=? ORDER BY run_at LIMIT 1 FOR UPDATE SKIP LOCKED", time.Now().UTC()).Scan(&msg.ID, &msg.Type, &msg.Payload)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, base.ErrNoTask
	}
	if err != nil {
		return nil, err
	}
	_, err = tx.ExecContext(ctx, "UPDATE tasks SET status = 'running' WHERE id = ?", msg.ID)
	if err != nil {
		return nil, err
	}
	err = tx.Commit()
	if err != nil {
		return nil, err
	}
	msg.Status = base.StatusRunning
	return msg, nil
}

func (s *Store) MarkSucceeded(ctx context.Context, id int64) error {
	res, err := s.db.ExecContext(ctx, "UPDATE tasks SET status=?,updated_at=NOW().UTC() WHERE id=? AND status=?", base.StatusSucceeded, id, base.StatusRunning)
	if err != nil {
		return fmt.Errorf("mark succeeded id=%d: %w", id, err)
	}
	rowscount, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("rows affected: %w", err)
	}
	if rowscount == 0 {
		return fmt.Errorf("no task in running: %w", base.ErrTaskNotRunning)
	}
	return nil
}

func (s *Store) MarkFailed(ctx context.Context, id int64, errMsg string) error {
	res, err := s.db.ExecContext(ctx, "UPDATE tasks SET attempts=attempts+1,last_error=?,status=IF(attempts>=max_attempts,?,?),run_at=NOW().UTC()+INTERVAL POW(2,attempts) SECOND,updated_at=NOW().UTC() WHERE id=? AND status=?", errMsg, base.StatusFailed, base.StatusPending, id, base.StatusRunning) //NOW()+2^n(second)
	if err != nil {
		return fmt.Errorf("mark failed id=%d: %w", id, err)
	}
	rowscount, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("rows affected: %w", err)
	}
	if rowscount == 0 {
		return fmt.Errorf("no task running: %w", base.ErrTaskNotRunning)
	}
	return nil
}
