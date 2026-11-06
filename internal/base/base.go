package base

import (
	"context"
	"errors"
	"time"
)

type Status string

const (
	StatusPending   Status = "pending"
	StatusRunning   Status = "running"
	StatusSucceeded Status = "succeeded"
	StatusFailed    Status = "failed"
)

var ErrNoTask = errors.New("kiln: no task available")

type TaskMessage struct {
	ID          int64
	Type        string
	Payload     []byte
	Status      Status
	Attempts    int
	MaxAttempts int
	RunAt       time.Time
	LastError   string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

type Store interface {
	Enqueue(ctx context.Context, msg *TaskMessage) error
	Claim(ctx context.Context) (*TaskMessage, error)
}
