package worker

import (
	"context"
	"crypto/rand"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"sync/atomic"
	"testing"
	"time"

	"github.com/CL0UD6LOCK/kiln/internal/base"
	"github.com/CL0UD6LOCK/kiln/internal/mysql"
	mysqldriver "github.com/go-sql-driver/mysql"
)

const consumeBenchmarkTasks = 5000

type consumeBenchmarkStore struct {
	base.Store
	finished       chan time.Time
	succeeded      atomic.Int64
	claimErrors    atomic.Int64
	claimDeadlocks atomic.Int64
	successErrors  atomic.Int64
}

func (s *consumeBenchmarkStore) Claim(ctx context.Context) (*base.TaskMessage, error) {
	msg, err := s.Store.Claim(ctx)
	if err != nil && !errors.Is(err, base.ErrNoTask) && ctx.Err() == nil {
		s.claimErrors.Add(1)
		var mysqlErr *mysqldriver.MySQLError
		if errors.As(err, &mysqlErr) && mysqlErr.Number == 1213 {
			s.claimDeadlocks.Add(1)
		}
	}
	return msg, err
}

func (s *consumeBenchmarkStore) MarkSucceeded(ctx context.Context, id int64, token string) error {
	if err := s.Store.MarkSucceeded(ctx, id, token); err != nil {
		s.successErrors.Add(1)
		return err
	}
	if s.succeeded.Add(1) == consumeBenchmarkTasks {
		s.finished <- time.Now()
	}
	return nil
}

func BenchmarkWorkerConsumeMySQL(b *testing.B) {
	dsn := os.Getenv("KILN_BENCH_DSN")
	if dsn == "" {
		b.Skip("KILN_BENCH_DSN not set")
	}
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		b.Fatal(err)
	}
	defer db.Close()

	// Claim can select any task in this schema. Keep benchmark processes from sharing it.
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	lockConn, err := db.Conn(ctx)
	if err != nil {
		b.Fatal(err)
	}
	defer lockConn.Close()
	var locked sql.NullBool
	err = lockConn.QueryRowContext(ctx, "SELECT GET_LOCK(CONCAT('kiln-bench:', MD5(DATABASE())), 0)").Scan(&locked)
	if err != nil {
		b.Fatalf("lock benchmark schema: %v", err)
	}
	if !locked.Valid || !locked.Bool {
		b.Fatal("benchmark requires its own MySQL schema. It is missing or another benchmark is using it")
	}
	// The lock stays with this session until the connection pool closes.

	for _, workers := range []int{1, 2, 4, 8} {
		b.Run(fmt.Sprintf("workers=%d", workers), func(b *testing.B) {
			b.StopTimer()
			var elapsed time.Duration
			var claimErrors, claimDeadlocks, successErrors int64
			for range b.N {
				duration, store := runConsumeBenchmarkBatch(b, db, workers)
				elapsed += duration
				claimErrors += store.claimErrors.Load()
				claimDeadlocks += store.claimDeadlocks.Load()
				successErrors += store.successErrors.Load()
			}
			batches := float64(b.N)
			b.ReportMetric(consumeBenchmarkTasks*batches/elapsed.Seconds(), "tasks/s")
			b.ReportMetric(consumeBenchmarkTasks, "succeeded/batch")
			b.ReportMetric(float64(claimErrors)/batches, "claim-errors/batch")
			b.ReportMetric(float64(claimDeadlocks)/batches, "claim-deadlocks/batch")
			b.ReportMetric(float64(successErrors)/batches, "write-errors/batch")
		})
	}
}

func runConsumeBenchmarkBatch(b *testing.B, db *sql.DB, workers int) (time.Duration, *consumeBenchmarkStore) {
	b.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	var hasTasks bool
	if err := db.QueryRowContext(ctx, "SELECT EXISTS(SELECT 1 FROM tasks)").Scan(&hasTasks); err != nil {
		b.Fatalf("check benchmark task table: %v", err)
	}
	if hasTasks {
		b.Fatal("benchmark requires an empty tasks table in a dedicated schema")
	}

	taskType := "kiln-bench-" + rand.Text()
	defer func() {
		cleanupCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		if _, err := db.ExecContext(cleanupCtx, "DELETE FROM tasks WHERE type = ?", taskType); err != nil {
			b.Errorf("clean up benchmark tasks: %v", err)
		}
	}()

	store := mysql.NewStore(db)
	for i := range consumeBenchmarkTasks {
		msg := &base.TaskMessage{Type: taskType, Payload: []byte("{}")}
		if err := store.Enqueue(ctx, msg); err != nil {
			b.Fatalf("enqueue task %d: %v", i+1, err)
		}
	}

	observed := &consumeBenchmarkStore{Store: store, finished: make(chan time.Time, 1)}
	pool := NewPool(observed, func(context.Context, *base.TaskMessage) error { return nil }, workers, 50*time.Millisecond)
	b.StartTimer()
	start := time.Now()
	pool.Start()
	var end time.Time
	select {
	case end = <-observed.finished:
	case <-ctx.Done():
	}
	b.StopTimer()
	pool.Shutdown()

	verifyCtx, cancelVerify := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancelVerify()
	var total, succeeded int
	err := db.QueryRowContext(verifyCtx,
		"SELECT COUNT(*), COUNT(CASE WHEN status = ? THEN 1 END) FROM tasks WHERE type = ?",
		base.StatusSucceeded, taskType).Scan(&total, &succeeded)
	if err != nil {
		b.Fatalf("verify benchmark tasks: %v", err)
	}
	if end.IsZero() || total != consumeBenchmarkTasks || succeeded != consumeBenchmarkTasks {
		b.Fatalf("incomplete batch: total=%d succeeded=%d claim_errors=%d claim_deadlocks=%d write_errors=%d",
			total, succeeded, observed.claimErrors.Load(), observed.claimDeadlocks.Load(), observed.successErrors.Load())
	}
	return end.Sub(start), observed
}
