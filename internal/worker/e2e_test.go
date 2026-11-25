package worker

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/CL0UD6LOCK/kiln/internal/base"
	"github.com/CL0UD6LOCK/kiln/internal/mysql"
	_ "github.com/go-sql-driver/mysql"
)

func TestE2E_MixedResults(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()
	db.Exec("DELETE FROM tasks")
	store := mysql.NewStore(db)
	for i := 0; i < 3; i++ {
		msg := &base.TaskMessage{
			Type:        "test",
			Payload:     []byte(`{}`),
			MaxAttempts: 2,
		}
		err := store.Enqueue(context.Background(), msg)
		if err != nil {
			t.Fatalf("Enqueue error: %v", err)
		}
	} //入队三个任务,MaxAttempts默认3
	h := func(ctx context.Context, msg *base.TaskMessage) error {
		if msg.ID%2 == 1 {
			return fmt.Errorf("intentional failure: odd ID")
		}
		return nil
	}
	pool := NewPool(store, h, 2, 50*time.Millisecond)
	pool.Start()
	defer pool.Shutdown()

	deadline := time.Now().Add(10 * time.Second)
	for {
		var n int
		err := db.QueryRowContext(
			context.Background(),
			"SELECT COUNT(*) FROM tasks WHERE status IN ('succeeded','failed')",
		).Scan(&n)

		if err != nil {
			t.Fatalf("count tasks error: %v", err)
		}
		if n == 3 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("timeout waiting for tasks, terminal count = %d", n)
		}
		time.Sleep(100 * time.Millisecond)
	}

	rows, err := db.QueryContext(context.Background(), "SELECT id,status,attempts,last_error FROM tasks ORDER BY id")
	if err != nil {
		t.Fatalf("querycontext error: %v", err)
	}
	defer rows.Close()
	count := 0
	for rows.Next() {
		var id int
		var status string
		var attempts int
		var lastErr sql.NullString
		err = rows.Scan(&id, &status, &attempts, &lastErr)
		if err != nil {
			t.Fatalf("rows scan error: %v", err)
		}
		if id%2 == 1 {
			if status != "failed" {
				t.Errorf("task %d: status = %q, want failed", id, status)
			}
			if attempts != 2 {
				t.Errorf("task %d: attempts = %d, want 2", id, attempts)
			}
			if lastErr.String != "intentional failure: odd ID" {
				t.Errorf("task %d: last_error = %q, want \"intentional failure: odd ID\"", id, lastErr.String)
			}
		} else {
			if status != "succeeded" {
				t.Errorf("task %d: status = %q, want succeeded", id, status)
			}
			if attempts != 0 {
				t.Errorf("task %d: attempts = %d, want 1", id, attempts)
			}
		}
		count++
	}
	if count != 3 {
		t.Fatal("tasks count error")
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
}

func TestWorker_HeartbeatPreventsDuplicateClaim(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()
	db.Exec("DELETE FROM tasks")
	store := mysql.NewStore(db)
	msg := &base.TaskMessage{
		Payload: []byte(`{}`),
	}
	err := store.Enqueue(context.Background(), msg)
	if err != nil {
		t.Fatalf("Enqueue error: %v", err)
	}
	started := make(chan *base.TaskMessage, 1)
	h := func(ctx context.Context, msg *base.TaskMessage) error {
		select {
		case started <- msg:
		case <-ctx.Done():
			return ctx.Err()
		}
		<-ctx.Done()
		return ctx.Err()
	}
	pool := NewPool(store, h, 1, 50*time.Millisecond)
	pool.Start()
	defer pool.Shutdown()
	var claimed *base.TaskMessage
	select {
	case claimed = <-started:
	case <-time.After(5 * time.Second):
		t.Fatalf("worker did not claim the task within 5 seconds")
	}
	t.Logf("task %d, initial lease until %v", claimed.ID, claimed.LeaseUntil)
	time.Sleep(time.Until(claimed.LeaseUntil.Add(time.Millisecond)))
	var leaseUntil time.Time
	var leaseToken string
	err = db.QueryRow("SELECT lease_until, lease_token FROM tasks WHERE id = ? AND status = ?", claimed.ID, base.StatusRunning).Scan(&leaseUntil, &leaseToken)
	if err != nil {
		t.Fatalf("read running task lease: %v", err)
	}
	if leaseToken != claimed.LeaseToken {
		t.Fatalf("worker A lost ownership of the task")
	}
	if !leaseUntil.After(time.Now()) {
		t.Fatalf("lease expired at %v", leaseUntil)
	}
	_, err = store.Claim(context.Background())
	if !errors.Is(err, base.ErrNoTask) {
		t.Fatalf("claim error: %v, want errnotask", err)
	}
}
