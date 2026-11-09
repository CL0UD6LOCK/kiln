package mysql

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"testing"
	"time"

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

func resetTasks(t *testing.T, db *sql.DB) {
	t.Helper() //报错时显示调用行
	if _, err := db.Exec("DELETE FROM tasks"); err != nil {
		t.Fatalf("reset tasks: %v", err)
	}
}

func TestClaim_ReturnsPendingTask(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()
	store := NewStore(db)
	msg := &base.TaskMessage{
		Payload: []byte(`{}`),
	}
	resetTasks(t, db)
	err := store.Enqueue(context.Background(), msg)
	if err != nil {
		t.Fatalf("Enqueue error: %v", err)
	}
	res, err := store.Claim(context.Background())
	if err != nil {
		t.Fatalf("Claim error: %v", err)
	}
	if res.ID != msg.ID {
		t.Fatalf("Error: ID=%d, want %d", res.ID, msg.ID)
	}
	if res.Status != base.StatusRunning {
		t.Fatalf("Error: set status failed")
	}
}

func TestClaim_EmptyQueue(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()
	store := NewStore(db)

	resetTasks(t, db)
	res, err := store.Claim(context.Background())
	if !errors.Is(err, base.ErrNoTask) || res != nil {
		t.Fatalf("Claim error")
	}
}

func TestMarkSucceeded(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()
	db.Exec("DELETE FROM tasks")
	store := NewStore(db)
	msg := &base.TaskMessage{
		Payload: []byte(`{}`),
	}
	err := store.Enqueue(context.Background(), msg)
	if err != nil {
		t.Fatalf("Enqueue error: %v", err)
	}
	res, err := store.Claim(context.Background())
	if err != nil {
		t.Fatalf("claim error: %v", err)
	}
	if res.ID != msg.ID {
		t.Fatalf("Error: ID=%d, want %d", res.ID, msg.ID)
	}
	err = store.MarkSucceeded(context.Background(), msg.ID)
	if err != nil {
		t.Fatalf("mark succeeded error: %v", err)
	}
	var status string
	err = db.QueryRowContext(context.Background(), "SELECT status FROM tasks WHERE id = ?", msg.ID).Scan(&status)
	if err != nil {
		t.Fatalf("queryrow error: %v", err)
	}
	if status != "succeeded" {
		t.Fatalf("status = %q, want succeeded", status)
	}
	err = store.MarkSucceeded(context.Background(), msg.ID) //再次marksucceedde，期望报错
	if !errors.Is(err, base.ErrTaskNotRunning) {
		t.Fatalf("second MarkSucceeded: got %v, want ErrTaskNotRunning", err)
	}
}

func TestMarkFailed_Retry(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()
	db.Exec("DELETE FROM tasks")
	store := NewStore(db)
	msg := &base.TaskMessage{
		Payload:     []byte(`{}`),
		MaxAttempts: 2,
	}
	err := store.Enqueue(context.Background(), msg)
	if err != nil {
		t.Fatalf("Enqueue error: %v", err)
	}
	res, err := store.Claim(context.Background())
	if err != nil {
		t.Fatalf("claim error: %v", err)
	}
	if res.ID != msg.ID {
		t.Fatalf("Error: ID=%d, want %d", res.ID, msg.ID)
	}
	err = store.MarkFailed(context.Background(), msg.ID, "boom")
	if err != nil {
		t.Fatalf("mark failed error: %v", err)
	}
	var status string
	var attempts int
	var lastErr string
	err = db.QueryRowContext(context.Background(), "SELECT attempts, status, last_error FROM tasks WHERE id = ?", msg.ID).Scan(&attempts, &status, &lastErr)
	if err != nil {
		t.Fatalf("queryrow error: %v", err)
	}
	if status != "pending" {
		t.Fatalf("status = %q, want pending", status)
	}
	if attempts != 1 {
		t.Fatalf("attempts = %d, want 1", attempts)
	}
	if lastErr != "boom" {
		t.Fatalf("last_error = %q, want boom", lastErr)
	}
	_, err = store.Claim(context.Background())
	if !errors.Is(err, base.ErrNoTask) { //指数退避未生效
		t.Fatalf("err = %v, want ErrNoTask", err)
	}
}

func TestMarkFailed_Final(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()
	db.Exec("DELETE FROM tasks")
	store := NewStore(db)
	msg := &base.TaskMessage{
		Payload:     []byte(`{}`),
		MaxAttempts: 2,
	}
	err := store.Enqueue(context.Background(), msg)
	if err != nil {
		t.Fatalf("Enqueue error: %v", err)
	}
	res, err := store.Claim(context.Background())
	if err != nil {
		t.Fatalf("claim error: %v", err)
	}
	if res.ID != msg.ID {
		t.Fatalf("Error: ID=%d, want %d", res.ID, msg.ID)
	}
	err = store.MarkFailed(context.Background(), msg.ID, "boom")
	if err != nil {
		t.Fatalf("mark failed error: %v", err)
	}
	var status string
	var attempts int
	var lastErr string
	err = db.QueryRowContext(context.Background(), "SELECT attempts, status, last_error FROM tasks WHERE id = ?", msg.ID).Scan(&attempts, &status, &lastErr)
	if err != nil {
		t.Fatalf("queryrow error: %v", err)
	}
	if status != "pending" {
		t.Fatalf("status = %q, want pending", status)
	}
	if attempts != 1 {
		t.Fatalf("attempts = %d, want 1", attempts)
	}
	if lastErr != "boom" {
		t.Fatalf("last_error = %q, want boom", lastErr)
	}
	_, err = db.Exec("UPDATE tasks SET run_at = ? WHERE id = ?", time.Now().UTC(), msg.ID) //老版本：(用Now()测试错误，怀疑可能时间存在精度bug，改为往前推 1s)，实则是时区不同的bug
	if err != nil {
		t.Fatalf("Exec error: %v", err)
	}
	// var runat_demo time.Time
	// var status_demo string //临时测试

	// err = db.QueryRow("SELECT run_at, status FROM tasks WHERE id = ?", msg.ID).Scan(&runat_demo, &status_demo)

	// if err != nil {
	// 	t.Fatalf("get run_at and status error: %v", err)
	// } //打印调试数据

	//fmt.Printf("\nrun_at: %v, status: %s\n", runat_demo, status_demo)
	//踩坑点，Claim和mysql操作没有同步时区，claim用的utc，mysql用的本地时区，导致bug，统一改为UTC时区
	res, err = store.Claim(context.Background())
	if err != nil {
		t.Fatalf("claim error: %v", err)
	}
	if res.ID != msg.ID {
		t.Fatalf("Error: ID=%d, want %d", res.ID, msg.ID)
	}
	err = store.MarkFailed(context.Background(), msg.ID, "boom2")
	if err != nil {
		t.Fatalf("mark failed error: %v", err)
	}
	err = db.QueryRowContext(context.Background(), "SELECT attempts, status, last_error FROM tasks WHERE id = ?", msg.ID).Scan(&attempts, &status, &lastErr)
	if err != nil {
		t.Fatalf("queryrow error: %v", err)
	}
	if status != "failed" {
		t.Fatalf("status = %q, want failed", status)
	}
	if attempts != 2 {
		t.Fatalf("attempts = %d, want 2", attempts)
	}
	if lastErr != "boom2" {
		t.Fatalf("last_error = %q, want boom2", lastErr)
	}
	_, err = store.Claim(context.Background())
	if !errors.Is(err, base.ErrNoTask) {
		t.Fatalf("err = %v, want ErrNoTask", err)
	}
	_, err = db.Exec("UPDATE tasks SET run_at = ? WHERE id = ?", time.Now().UTC(), msg.ID)
	if err != nil {
		t.Fatalf("Exec error: %v", err)
	}
	_, err = store.Claim(context.Background())
	if !errors.Is(err, base.ErrNoTask) {
		t.Fatalf("err = %v, want ErrNoTask", err)
	}
}
