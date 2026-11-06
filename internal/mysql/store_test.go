package mysql

import (
	"context"
	"database/sql"
	"os"
	"testing"

	"github.com/CL0UD6LOCK/kiln/internal/base"
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

func TestEnqueue_Defaults(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()
	store := NewStore(db)

	msg := &base.TaskMessage{
		Type:    "email:send",
		Payload: []byte(`{"to":"a@b.com"}`),
	}
	err := store.Enqueue(context.Background(), msg)

	if err != nil {
		t.Fatalf("Enqueue error: %v", err)
	}
	if msg.ID <= 0 {
		t.Fatalf("ID %d invalid", msg.ID)
	}
	if msg.MaxAttempts != 3 {
		t.Fatalf("MaxAttempts %d error", msg.MaxAttempts)
	}
}

func TestEnqueue_KeyValue(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()
	store := NewStore(db)
	msg := &base.TaskMessage{
		Type:        "email:send",
		Payload:     []byte(`{}`),
		MaxAttempts: 5,
	}
	err := store.Enqueue(context.Background(), msg)
	if err != nil {
		t.Fatalf("Enqueue error: %v", err)
	}
	if msg.ID <= 0 {
		t.Fatalf("ID %d invalid", msg.ID)
	}
	if msg.MaxAttempts != 5 {
		t.Fatalf("MaxAttempts %d error", msg.MaxAttempts)
	}
	if msg.RunAt.IsZero() {
		t.Fatalf("RunAt time error: null")
	}
}

func TestEnqueue_InvalidJSON(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()
	store := NewStore(db)
	msg := &base.TaskMessage{
		Payload: []byte("not json"),
	}
	err := store.Enqueue(context.Background(), msg)
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	if msg.ID != 0 {
		t.Fatalf("ID %d invalid", msg.ID)
	}
}

func TestClaim_ReturnsPendingTask(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()
	store := NewStore(db)
	msg := &base.TaskMessage{}
	err := store.Enqueue(context.Background(), msg)
	if err != nil {
		t.Fatalf("Enqueue error: %v", err)
	}
	res, err := store.Claim(context.Background())
	if err != nil {
		t.Fatalf("Claim error: %v", err)
	}
	if res.ID != msg.ID {
		t.Fatalf("Error: id changed")
	}
	if res.Status != base.StatusRunning {
		t.Fatalf("Error: set status failed")
	}
}
