package worker

import (
	"context"
	"errors"
	"time"

	"github.com/CL0UD6LOCK/kiln/internal/base"
)

type Handler func(ctx context.Context, msg *base.TaskMessage) error

type worker struct {
	store        base.Store
	handler      Handler
	pollInterval time.Duration
}

func (w *worker) run(ctx context.Context) {
	msg, err := w.store.Claim(ctx)
	if errors.Is(err, base.ErrNoTask) {

	}
	if err != nil {

	}
	for {

	}
}
