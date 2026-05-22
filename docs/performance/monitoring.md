# new-api 性能监控与排查（运行中）

本项目已内置多套性能观测能力：系统监控（CPU/内存/磁盘）、Relay 性能聚合指标、pprof/trace 采样以及可选的 Pyroscope 持续 Profiling。

## 1) 先看“现象”与“范围”

- 系统状态（Root）：`GET /api/performance/stats`
- Relay 聚合指标（延迟/TTFT/成功率/TPS）：`GET /api/perf-metrics/summary`、`GET /api/perf-metrics?model=...&hours=...`

通常先用上面两类数据定位：是“单个模型/分组慢”，还是“整机资源打满”，还是“全局失败率升高”。

## 2) Root 受控的 pprof / trace 下载接口（推荐）

这些接口都挂在 Root 权限下：`/api/performance/pprof/*`，便于在生产环境内网下载 profile（避免单独暴露 `:8005`）。

- CPU：`GET /api/performance/pprof/cpu?seconds=10`（1–120 秒）
- Heap：`GET /api/performance/pprof/heap?gc=false`
- Goroutine：`GET /api/performance/pprof/goroutine?debug=2`
- Mutex：`GET /api/performance/pprof/mutex`
- Block：`GET /api/performance/pprof/block`
- Trace：`GET /api/performance/pprof/trace?seconds=5`（1–60 秒）

分析命令示例：

```bash
go tool pprof -top ./cpu-*.pprof
go tool pprof -http=:0 ./cpu-*.pprof
go tool pprof -http=:0 ./heap-*.pprof
go tool trace ./trace-*.out
```

注意：Go 运行时同一时刻只能有一个 CPU profile/trace 在进行；如果你开启了 Pyroscope（或其他 profiler），CPU/trace 采样可能返回“already in use”一类冲突错误。

## 3) 传统 pprof 端口（仅内网/本机）

设置环境变量 `ENABLE_PPROF=true` 时，进程会监听 `0.0.0.0:8005`（`net/http/pprof` 默认 mux）。

生产环境务必只在内网开放或通过隧道/端口转发访问，避免直接暴露到公网。

## 4) 自动采样（高负载时落盘）

当 `ENABLE_PPROF=true` 且监控开关启用时，进程会在资源超过阈值时自动写入 profile 文件（默认 `./pprof/`）：

- CPU 超阈值：写入 `cpu-YYYYMMDDhhmmss.pprof`
- 内存超阈值：写入 `heap-YYYYMMDDhhmmss.pprof`

可选环境变量：

- `PPROF_OUTPUT_DIR`：profile 输出目录（默认 `./pprof`）
- `PPROF_MONITOR_INTERVAL_SECONDS`：检测间隔（默认 `30`）
- `PPROF_MONITOR_COOLDOWN_SECONDS`：写文件冷却时间（默认 `300`）
- `PPROF_CPU_DURATION_SECONDS`：CPU profile 时长（默认 `10`）
- `PPROF_HEAP_GC_BEFORE`：heap 前是否 `runtime.GC()`（默认 `false`）

阈值来源：`performance_setting.monitor_*_threshold`（Root → System Settings → Performance）。

## 5) 持续 Profiling（Pyroscope，可选）

配置 `PYROSCOPE_URL` 后会自动上报多种 profile（CPU/Heap/Goroutine/Mutex/Block）。适合长期观察“哪个函数在烧 CPU/分配内存/锁竞争”等趋势。

