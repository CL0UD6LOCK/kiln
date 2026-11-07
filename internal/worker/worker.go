package worker

import (
	"context"
	"errors"
	"log"
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
	for {
		msg, err := w.store.Claim(ctx)
		if err != nil {
			if !errors.Is(err, base.ErrNoTask) {
				if ctx.Err() == nil {
					log.Printf("kiln: claim failed: %v", err)
				}
			}
			select {
			case <-ctx.Done():
				return
			case <-time.After(w.pollInterval):
				continue //空队列是正常情况
			}
		}
		if err := w.handler(ctx, msg); err != nil {
			log.Printf("kiln: task %d failed: %v", msg.ID, err)
		}
	}
}
