package worker

import (
	"context"
	"database/sql"
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
