package worker

import (
	"context"
	"database/sql"
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
}
