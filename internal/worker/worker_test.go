package worker

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/CL0UD6LOCK/kiln/internal/base"
	"github.com/CL0UD6LOCK/kiln/internal/mysql"

	_ "github.com/go-sql-driver/mysql"
)

func openTestDB(t *testing.T) *sql.DB {
	dsn := os.Getenv("KILN_TEST_DSN")
	if dsn == "" {
		t.Skip("KILN_TEST_DSN not set")
	}
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		t.Fatal(err)
	}
	return db
}

func TestPool_ProcessesAllTasks(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()
	db.Exec("DELETE FROM tasks")
	store := mysql.NewStore(db)

	want := map[int64]bool{}
	for i := 0; i < 3; i++ {
		msg := &base.TaskMessage{
			Type:    "test",
			Payload: []byte(`{}`),
		}
		err := store.Enqueue(context.Background(), msg)
		if err != nil {
			t.Fatalf("Enqueue error: %v", err)
		}
		want[msg.ID] = true
	}
	done := make(chan int64, 3)
	h := func(ctx context.Context, msg *base.TaskMessage) error {
		done <- msg.ID
		return nil
	}

	pool := NewPool(store, h, 2, 50*time.Millisecond)
	pool.Start()
	defer pool.Shutdown()

	for i := 0; i < 3; i++ {
		select {
		case id := <-done:
			if !want[id] {
				t.Fatalf("got unexpected task id %d", id)
			}
			delete(want, id)
		case <-time.After(5 * time.Second):
			t.Fatalf("timeout: %d tasks not processed", len(want))
		}
	}
	waitCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	for {
		var n int
		err := db.QueryRowContext(waitCtx, "SELECT COUNT(*) FROM tasks WHERE status = ?", base.StatusSucceeded).Scan(&n)
		if err != nil {
			t.Fatalf("status succeeded tasks queryrow error: %v", err)
		}
		if n == 3 {
			break
		}
		select {
		case <-waitCtx.Done():
			t.Fatalf("wait timeout: %d/3 succeeded", n)
		case <-time.After(50 * time.Millisecond):
			continue
		}
	}
}

type leaseLostStore struct {
	base.Store
	claimed bool
}

func (s *leaseLostStore) Claim(ctx context.Context) (*base.TaskMessage, error) {
	if s.claimed {
		return nil, base.ErrNoTask
	}
	s.claimed = true
	return &base.TaskMessage{ID: 1, LeaseToken: "test-token"}, nil
}
func (s *leaseLostStore) Heartbeat(ctx context.Context, id int64, token string) error {
	return base.ErrTaskNotRunning
}
func TestWorker_CancelsHandlerOnLeaseLoss(t *testing.T) {
	observed := make(chan error, 1)

	handler := func(ctx context.Context, msg *base.TaskMessage) error {
		<-ctx.Done()
		observed <- ctx.Err()
		return ctx.Err()
	}
	pool := NewPool(&leaseLostStore{}, handler, 1, 50*time.Millisecond)
	pool.Start()
	defer pool.Shutdown()
	select {
	case err := <-observed:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("handler returned %v, want context.Cancelled", err)
		}
	case <-time.After(15 * time.Second):
		t.Fatalf("handler did not observe cancellation within 15 seconds")
	}

}
