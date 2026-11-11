package worker

import (
	"context"
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
	//res, err := db.ExecContext(context.Background(), "SELECT * FROM tasks WHERE status = failed OR status = succeeded")
	// TODO: handler 按 ID 奇偶决定成功/失败, MaxAttempts 设 2
	// TODO: NewPool + Start, defer Shutdown
	// TODO: 轮询 DB 直到终态任务数 == 3, 超时就 Fatal
	// TODO: 断言每个任务的 status / attempts / last_error
}
