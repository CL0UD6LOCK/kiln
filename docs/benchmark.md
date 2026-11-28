# MySQL worker benchmark · MySQL worker 压测

## Three-run comparison · 三轮对照

Run on 2026-11-28 at revision `7e6c20f` with Go 1.27.0 on Windows amd64 (Intel Core Ultra 9 185H). All 12 batches ran sequentially on the same local MySQL 8.0.46 instance, grouped by worker count. MySQL used `REPEATABLE-READ`, binary logging, `innodb_flush_log_at_trx_commit=1`, and `sync_binlog=1`.

2026-11-28 在 `7e6c20f` 上测得。环境为 Windows amd64（Intel Core Ultra 9 185H）、Go 1.27.0 和同一个本机 MySQL 8.0.46 实例。按 worker 数分组顺序运行，每档三轮。MySQL 使用 `REPEATABLE-READ`，启用 binlog，`innodb_flush_log_at_trx_commit` 和 `sync_binlog` 均为 `1`。

| Workers / worker 数 | Median tasks/s / 吞吐中位数 | Min–max tasks/s / 吞吐范围 | Claim deadlocks (1213) by run / 各轮领取死锁 |
| ---: | ---: | ---: | ---: |
| 1 | 81.43 | 80.72–158.0 | 0 / 0 / 0 |
| 2 | 90.86 | 90.47–91.81 | 42 / 58 / 57 |
| 4 | 107.6 | 105.8–110.1 | 146 / 116 / 160 |
| 8 | 141.4 | 140.7–142.2 | 197 / 171 / 173 |

All 60,000 tasks reached `succeeded`, with zero success write errors. All 1,120 claim errors were MySQL 1213 deadlocks, counted per failed claim attempt. The task table was empty after the final batch.

共 60,000 个任务成功，成功写回错误为 0。1,120 次领取错误均为 MySQL 1213，按失败的领取尝试计数。最后一轮结束后任务表为空。

Eight workers had the highest median throughput in this session. Single-worker results varied from 80.72 to 158.0 tasks/s. These measurements describe one local run series.

本次 8 worker 的吞吐中位数最高。单 worker 的吞吐在 80.72–158.0 个任务/秒之间波动，结果反映本机这批测试。

### Per-run results · 逐轮结果

| Workers / worker 数 | Run / 轮次 | Time / 耗时（秒） | Succeeded / 成功数 | Tasks/s / 吞吐 | Claim deadlocks / 领取死锁（1213） |
| ---: | ---: | ---: | ---: | ---: | ---: |
| 1 | 1 | 31.65 | 5,000 | 158.0 | 0 |
| 1 | 2 | 61.40 | 5,000 | 81.43 | 0 |
| 1 | 3 | 61.94 | 5,000 | 80.72 | 0 |
| 2 | 1 | 55.27 | 5,000 | 90.47 | 42 |
| 2 | 2 | 54.46 | 5,000 | 91.81 | 58 |
| 2 | 3 | 55.03 | 5,000 | 90.86 | 57 |
| 4 | 1 | 47.28 | 5,000 | 105.8 | 146 |
| 4 | 2 | 45.40 | 5,000 | 110.1 | 116 |
| 4 | 3 | 46.48 | 5,000 | 107.6 | 160 |
| 8 | 1 | 35.17 | 5,000 | 142.2 | 197 |
| 8 | 2 | 35.35 | 5,000 | 141.4 | 171 |
| 8 | 3 | 35.55 | 5,000 | 140.7 | 173 |

## Initial single-worker run · 首轮单 worker 基线

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

## Run the benchmark / 运行压测

Create a dedicated MySQL database for the [benchmark](../internal/worker/benchmark_test.go), apply the [repository migration](../migrations/001_create_tasks.sql), and leave the task table empty. Set `KILN_BENCH_DSN` to its MySQL DSN. The benchmark cleans up its tasks after each batch.

为[压测代码](../internal/worker/benchmark_test.go)新建专用 MySQL 库，应用[仓库迁移](../migrations/001_create_tasks.sql)，并保持任务表为空。将 `KILN_BENCH_DSN` 指向该库。每批结束后程序会清理本批任务。

```sh
go test -run '^$' -bench '^BenchmarkWorkerConsumeMySQL$' -benchtime=1x -count=3 -timeout 30m ./internal/worker
```

The command runs three batches of 5,000 tasks each for 1, 2, 4, and 8 workers.

命令分别用 1、2、4、8 个 worker 各跑三轮，每轮 5,000 个任务。

## Method

- Ran on Windows amd64 with Go 1.27.0 and a fresh local MySQL 8.0.46 database initialized with the [repository migration](../migrations/001_create_tasks.sql). Both `innodb_flush_log_at_trx_commit` and `sync_binlog` were set to `1`.
- Enqueued 5,000 tasks through the MySQL store before starting the worker pool. Each handler call immediately returned `nil`. The polling interval was 50 ms.
- Timed from pool startup through the final successful database write. After shutdown, a database query verified the final status of all 5,000 tasks. Normal empty-queue responses were excluded from claim errors.

## 测试方法

- 在 Windows amd64 上使用 Go 1.27.0 和全新的本机 MySQL 8.0.46 测试库，应用[仓库迁移](../migrations/001_create_tasks.sql)。`innodb_flush_log_at_trx_commit` 和 `sync_binlog` 均为 `1`。
- 通过 MySQL Store 预先入队 5,000 个任务。handler 立即返回 `nil`，worker 轮询间隔为 50 毫秒。
- 从 Pool 启动计时到最后一次成功写回。停机后查询数据库确认所有任务状态。队列为空属于正常响应，不计为领取错误。
