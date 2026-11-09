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
var ErrTaskNotRunning = errors.New("kiln: task not running")

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
	MarkSucceeded(ctx context.Context, id int64) error
	MarkFailed(ctx context.Context, id int64, errMsg string) error
}
