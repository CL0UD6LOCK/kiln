package worker

import (
	"context"
	"errors"
	"log"
	"sync"
	"time"

	"github.com/CL0UD6LOCK/kiln/internal/base"
)

type Handler func(ctx context.Context, msg *base.TaskMessage) error

type worker struct {
	store        base.Store
	handler      Handler
	pollInterval time.Duration
}

type Pool struct {
	store        base.Store
	handler      Handler
	concurrency  int
	pollInterval time.Duration
	cancel       context.CancelFunc
	wg           sync.WaitGroup
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

func NewPool(store base.Store, h Handler, concurrency int, pollInterval time.Duration) *Pool {
	return &Pool{
		store:        store,
		handler:      h,
		concurrency:  concurrency,
		pollInterval: pollInterval,
	}
}

func (p *Pool) Start() {
	ctx, cancel := context.WithCancel(context.Background())
	p.cancel = cancel
	for i := 0; i < p.concurrency; i++ {
		w := &worker{
			store:        p.store,
			handler:      p.handler,
			pollInterval: p.pollInterval,
		}
		p.wg.Add(1)
		go func() {
			defer p.wg.Done()
			w.run(ctx)
		}()
	}
}

func (p *Pool) Shutdown() {
	p.cancel()
	p.wg.Wait()
}
