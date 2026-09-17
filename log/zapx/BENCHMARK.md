# Zap 与 Sphere log 性能对比

测量日期：2026-09-17。结果来自本机微基准，不代表文件系统或终端吞吐量。

采样发生在提交前恢复 `error` 字段的特殊编码分支之前；性能表保留当时的对照结果，未对该兼容性收尾重新采样。

## 修改与验证

`Backend.Log` 在转换字段前调用 Zap Core 的 `Enabled`，关闭级别时直接返回。未知级别先归一化为 Info，维持原来的输出行为；每次查询当前级别，因此仍支持动态调级。没有更改公共接口。

回归测试先在优化前复现了「关闭 Debug 仍执行 LogValuer」的问题，优化后通过，并验证动态开启 Debug 与未知级别回退。`make check` 已通过（依赖检查、格式、vet、golangci-lint、nilaway、全仓测试）。

## 测量方式

- Apple M1 Pro，darwin/arm64，Go 1.27.1；模块声明 Go 1.26.8，Zap v1.28.0。
- `-cpu=1`，每个子基准 `-benchtime=200ms`；三轮交错执行优化前后版本，每轮各两次，共六个样本，表中取中位数。顺序为 before→after、after→before、before→after。
- 两边使用相同的 Production JSON 编码器，时间使用 ISO8601，耗时单位是秒，最低级别是 Info。只有 Caller 场景启用调用位置；不启用采样或自动堆栈。
- 写入 `io.Discard`，计入参数构造、字段转换、编码和分配；不计真实磁盘/终端 I/O。没有使用 NopCore 来跳过编码。
- Logger、SugaredLogger 和持久字段只在计时外创建；每次日志调用的动态字段在计时内创建。
- 普通场景使用原生 `*zap.Logger`；格式化场景使用预先创建的 `*zap.SugaredLogger`。原生分组使用 `zap.Dict`，封装分组使用 `log.Group`。
- Sphere 默认使用 `log.NewLogger(zapxBackend)`；GlobalFields3 单独使用包级 `log.Info`。单后端，没有 context 字段提取、多后端 fan-out 或并发压测。
- 原生路径直接调用日志方法，没有额外使用 `logger.Check` 在调用点跳过参数构造。测试结果不代表手动优化到极致的 Zap 用法。
- 优化前对照保留了会话开始时已有的未提交修改，只撤去本次提前级别判断，通过 Go overlay 编译；不拿 HEAD 的旧分组实现混入对照。
- 百分比是样本中位数的比值，不是统计显著性检验。微小变化不应单独作为优化依据。

## 裸 Zap 与优化后的封装

时间越低越好；`B/op` 是每条日志的分配字节数，`allocs/op` 是分配次数。

| 场景 | Zap ns/op | Sphere ns/op | Sphere 耗时变化 | Zap / Sphere B/op | Zap / Sphere allocs/op |
|---|---:|---:|---:|---:|---:|
| 仅消息 | 363.7 | 399.8 | +9.9% | 0 / 0 | 0 / 0 |
| 实例：3 个字段 | 622.0 | 803.2 | +29.1% | 192 / 320 | 1 / 2 |
| 全局 log.Info：3 个字段 | 635.8 | 813.4 | +27.9% | 192 / 320 | 1 / 2 |
| 10 个字段 | 1024.5 | 1427.5 | +39.3% | 704 / 1120 | 1 / 2 |
| 分组（含耗时、错误） | 749.3 | 903.5 | +20.6% | 280 / 264 | 3 / 4 |
| caller + 1 个字段 | 984.9 | 1158.5 | +17.6% | 336 / 384 | 3 / 4 |
| 2 个持久字段 + 1 个动态字段 | 488.9 | 593.3 | +21.3% | 64 / 112 | 1 / 2 |
| Infof | 543.9 | 593.0 | +9.0% | 48 / 80 | 1 / 2 |
| 关闭的 Debug：3 个字段 | 94.4 | 88.3 | -6.5% | 192 / 128 | 1 / 1 |
| 关闭的 Debug：分组 | 124.5 | 101.8 | -18.3% | 216 / 128 | 3 / 2 |
| 关闭的 Debug：延迟结构化值 | 61.5 | 63.7 | +3.6% | 64 / 48 | 1 / 1 |
| 关闭的 Debugf | 29.8 | 143.0 | +379.5% | 0 / 80 | 0 / 2 |

正常输出的结构化日志仍有 Attr→zap.Field 的转换成本，常见的非空字段场景多一次分配。关闭级别后，转换数组不再创建；此时测试中 Sphere 的参数数组比 Zap 的参数数组小，部分场景反而更快，不能把这个结论泛化到所有调用方式。

## 本次优化前后（仅 Sphere）

| 场景 | 优化前 ns/op | 优化后 ns/op | 耗时变化 | B/op 前→后 | allocs/op 前→后 |
|---|---:|---:|---:|---:|---:|
| 仅消息 | 393.8 | 399.8 | +1.5% | 0 → 0 | 0 → 0 |
| 实例：3 个字段 | 786.9 | 803.2 | +2.1% | 320 → 320 | 2 → 2 |
| 全局 log.Info：3 个字段 | 774.0 | 813.4 | +5.1% | 320 → 320 | 2 → 2 |
| 10 个字段 | 1328.5 | 1427.5 | +7.5% | 1120 → 1120 | 2 → 2 |
| 分组（含耗时、错误） | 870.0 | 903.5 | +3.9% | 264 → 264 | 4 → 4 |
| caller + 1 个字段 | 1146.0 | 1158.5 | +1.1% | 384 → 384 | 4 → 4 |
| 2 个持久字段 + 1 个动态字段 | 562.8 | 593.3 | +5.4% | 112 → 112 | 2 → 2 |
| Infof | 559.9 | 593.0 | +5.9% | 80 → 80 | 2 → 2 |
| 关闭的 Debug：3 个字段 | 209.8 | 88.3 | -57.9% | 320 → 128 | 2 → 1 |
| 关闭的 Debug：分组 | 174.1 | 101.8 | -41.6% | 216 → 128 | 4 → 2 |
| 关闭的 Debug：延迟结构化值 | 182.0 | 63.7 | -65.0% | 216 → 48 | 4 → 1 |
| 关闭的 Debugf | 162.0 | 143.0 | -11.7% | 80 → 80 | 2 → 2 |

提前检查主要改善被级别过滤的结构化日志。正常输出时多了一次 Enabled 查询，不会减少字段转换的分配，也不保证更快。这里把正常输出的前后数据一并列出，避免只展示收益场景。

`Debugf` 仍在进入 Backend 前执行 `fmt.Sprintf`，因此关闭级别时仍有格式化和分配；本次优化没有改动这条上层调用链。业务参数表达式、`log.Group` 的构造、外层 context 字段提取也不能被后端的提前返回省掉。

## 各场景实际输出

以下内容由同一组场景的输出测试采集。除了真实时间戳与调用行号，测试逐项校验两边 JSON 内容相等；关闭级别的场景校验两边都没有输出。这里只验证这些场景，不代表两套 API 的所有边界行为都相同。

### Message — 仅消息

Zap:

```json
{"level":"info","timestamp":"2026-09-17T17:12:24.939+0800","msg":"request completed"}
```

Sphere:

```json
{"level":"info","timestamp":"2026-09-17T17:12:24.939+0800","msg":"request completed"}
```

### Fields3 — 实例：3 个字段

Zap:

```json
{"level":"info","timestamp":"2026-09-17T17:12:24.939+0800","msg":"request completed","method":"GET","status":200,"duration":0.005}
```

Sphere:

```json
{"level":"info","timestamp":"2026-09-17T17:12:24.939+0800","msg":"request completed","method":"GET","status":200,"duration":0.005}
```

### GlobalFields3 — 全局 log.Info：3 个字段

Zap:

```json
{"level":"info","timestamp":"2026-09-17T17:12:24.939+0800","msg":"request completed","method":"GET","status":200,"duration":0.005}
```

Sphere:

```json
{"level":"info","timestamp":"2026-09-17T17:12:24.939+0800","msg":"request completed","method":"GET","status":200,"duration":0.005}
```

### Fields10 — 10 个字段

Zap:

```json
{"level":"info","timestamp":"2026-09-17T17:12:24.940+0800","msg":"request completed","method":"GET","path":"/users","status":200,"duration":0.005,"request_id":"req-123","user_id":42,"cached":false,"bytes":1024,"region":"sg","sample":0.5}
```

Sphere:

```json
{"level":"info","timestamp":"2026-09-17T17:12:24.940+0800","msg":"request completed","method":"GET","path":"/users","status":200,"duration":0.005,"request_id":"req-123","user_id":42,"cached":false,"bytes":1024,"region":"sg","sample":0.5}
```

### Group — 分组（含耗时、错误）

Zap:

```json
{"level":"info","timestamp":"2026-09-17T17:12:24.940+0800","msg":"request failed","request":{"method":"GET","duration":0.005,"error":"upstream timeout"}}
```

Sphere:

```json
{"level":"info","timestamp":"2026-09-17T17:12:24.940+0800","msg":"request failed","request":{"method":"GET","duration":0.005,"error":"upstream timeout"}}
```

### Caller — caller + 1 个字段

Zap:

```json
{"level":"info","timestamp":"2026-09-17T17:12:24.940+0800","caller":"zapx/benchmark_test.go:96","msg":"request completed","status":200}
```

Sphere:

```json
{"level":"info","timestamp":"2026-09-17T17:12:24.940+0800","caller":"zapx/benchmark_test.go:97","msg":"request completed","status":200}
```

### WithFields — 2 个持久字段 + 1 个动态字段

Zap:

```json
{"level":"info","timestamp":"2026-09-17T17:12:24.940+0800","msg":"request completed","service":"api","version":"v1","status":200}
```

Sphere:

```json
{"level":"info","timestamp":"2026-09-17T17:12:24.940+0800","msg":"request completed","service":"api","version":"v1","status":200}
```

### Formatted — Infof

Zap:

```json
{"level":"info","timestamp":"2026-09-17T17:12:24.940+0800","msg":"request req-123 completed with 200"}
```

Sphere:

```json
{"level":"info","timestamp":"2026-09-17T17:12:24.940+0800","msg":"request req-123 completed with 200"}
```

### DisabledFields3 — 关闭的 Debug：3 个字段

Zap：无输出。

Sphere：无输出。

### DisabledGroup — 关闭的 Debug：分组

Zap：无输出。

Sphere：无输出。

### DisabledLazyValue — 关闭的 Debug：延迟结构化值

Zap：无输出。

Sphere：无输出。

### DisabledFormatted — 关闭的 Debugf

Zap：无输出。

Sphere：无输出。

## 复现当前版本

```sh
go test ./log/zapx -run '^$' -bench '^BenchmarkLoggerComparison$' -benchmem -benchtime=200ms -count=6 -cpu=1
go test ./log/zapx -run TestLoggerComparisonOutput -v -count=1
make check
```

基准与输出校验位于 `benchmark_test.go`；级别过滤回归测试位于 `level_test.go`。优化前数据来自本次会话保存的后端源码快照，复现时需用对应快照 overlay 编译。
