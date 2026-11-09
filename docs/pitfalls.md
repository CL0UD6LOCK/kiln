# Pitfalls / 踩坑记录

## 1. Claim 领不到刚改过 run_at 的任务（时区不一致）
- **日期**：2026-11-09
- **现象**：TestMarkFailed_Final() 中，UPDATE run_at = NOW() 后再 Claim，返回 ErrNoTask
- **排查**：
  1. 怀疑时钟精度问题，往前推 1s → 仍失败
  2. 打印 run_at 和 status → run_at 显示 14:07 +0000 UTC，但实际 UTC 是 06:07
- **原因**：MySQL NOW() 用服务器本地时区（UTC+8），Go 传入的 time.Now() 是 UTC，驱动 parseTime 又把读到的时间当 UTC → 时差 8 小时
- **影响**：生产中每次重试会被多推迟 8 小时
- **修复**：Go 统一传 time.Now().UTC()；DSN 加 time_zone='+00:00'，让 CURRENT_TIMESTAMP 默认值也是 UTC；README 写明要求
- **经验**：时间只用一个时钟来源/一个时区,排查时先把实际取值打印出来调试

store_test.go 258行调试代码&注释：
	var runat_demo time.Time
	var status_demo string //临时测试

	err = db.QueryRow("SELECT run_at, status FROM tasks WHERE id = ?", msg.ID).Scan(&runat_demo, &status_demo)

	if err != nil {
	t.Fatalf("get run_at and status error: %v", err)
	 } //打印调试数据

	fmt.Printf("\nrun_at: %v, status: %s\n", runat_demo, status_demo)