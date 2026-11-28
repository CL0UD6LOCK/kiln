# MySQL worker benchmark · MySQL worker 压测

Recorded on 2026-11-28 at revision [`8fd2434`](https://github.com/CL0UD6LOCK/kiln/commit/8fd2434c01e4c525c77c89f2257b81599c98e7fa).

| Metric / 指标 | Result / 结果 |
| --- | ---: |
| Workers / worker 数 | 1 |
| Runs / 测试轮数 | 1 |
| Tasks confirmed as `succeeded` / 确认成功的任务 | 5,000 / 5,000 |
| Elapsed / 消费耗时 | 44.83 s |
| Throughput / 吞吐 | 111.54 tasks/s |
| Claim errors / 领取错误 | 0 |
| MySQL deadlocks (1213) during claims / 领取时的 MySQL 死锁（1213） | 0 |
| Success write errors / 成功写回错误 | 0 |

## Method

- Ran on Windows amd64 with Go 1.27.0 and a fresh local MySQL 8.0.46 database initialized with the [repository migration](../migrations/001_create_tasks.sql). Both `innodb_flush_log_at_trx_commit` and `sync_binlog` were set to `1`.
- Enqueued 5,000 tasks through the MySQL store before starting the worker pool. Each handler call immediately returned `nil`. The polling interval was 50 ms.
- Timed from pool startup through the final successful database write. After shutdown, a database query verified the final status of all 5,000 tasks. Normal empty-queue responses were excluded from claim errors.

## 测试方法

- 在 Windows amd64 上使用 Go 1.27.0 和全新的本机 MySQL 8.0.46 测试库，应用[仓库迁移](../migrations/001_create_tasks.sql)。`innodb_flush_log_at_trx_commit` 和 `sync_binlog` 均为 `1`。
- 通过 MySQL Store 预先入队 5,000 个任务。handler 立即返回 `nil`，worker 轮询间隔为 50 毫秒。
- 从 Pool 启动计时到最后一次成功写回。停机后查询数据库确认所有任务状态。队列为空属于正常响应，不计为领取错误。
