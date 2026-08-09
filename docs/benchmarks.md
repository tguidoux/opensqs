# OpenSQS Performance Benchmarks

Baseline performance benchmarks for the OpenSQS message queue system.

## Environment

- **OS**: macOS (Darwin)
- **Architecture**: arm64
- **CPU**: Apple M4 Max
- **Go Version**: 1.25.5
- **Benchmark Mode**: 100 fixed iterations (`-benchtime=100x`)

## Running Benchmarks

```bash
# Memory store benchmarks
bazel test --cache_test_results=no \
  --test_arg=-test.bench=. --test_arg=-test.benchtime=100x \
  --test_arg=-test.run=^$ --test_output=all \
  //pkgs/v1/queue/store/memory/tests:go_default_test

# SQLite store benchmarks
bazel test --cache_test_results=no \
  --test_arg=-test.bench=. --test_arg=-test.benchtime=100x \
  --test_arg=-test.run=^$ --test_output=all \
  //pkgs/v1/queue/store/sqlite/tests:go_default_test

# BadgerDB store benchmarks (when available)
# bazel test --cache_test_results=no \
#   --test_arg=-test.bench=. --test_arg=-test.benchtime=100x \
#   --test_arg=-test.run=^$ --test_output=all \
#   //pkgs/v1/queue/store/badger/tests:go_default_test

# Handler pipeline benchmarks
bazel test --cache_test_results=no \
  --test_arg=-test.bench=. --test_arg=-test.benchtime=100x \
  --test_arg=-test.run=^$ --test_output=all \
  //apps/go/server/handlers/tests:go_default_test
```

## Memory Store Baseline Results

| Benchmark | Time/op | Memory/op | Allocs/op |
|-----------|---------|-----------|-----------|
| `BenchmarkSendMessage` | 309.6 ns | 466 B | 3 |
| `BenchmarkReceiveMessage` | 2,208 ns | 3,045 B | 27 |
| `BenchmarkDeleteMessage` | 36.25 ns | 0 B | 0 |
| `BenchmarkSendMessageBatch10` | 1,468 ns | 4,497 B | 37 |
| `BenchmarkConcurrentSendReceive` | 345.4 ns | 479 B | 3 |

### Key Observations

- **SendMessage**: ~310 ns per message — fast in-memory map insertion with receipt handle generation.
- **ReceiveMessage**: ~2.2 µs per message — includes visibility timeout management, receipt handle generation, and FIFO group tracking.
- **DeleteMessage**: ~36 ns — the fastest operation, a simple map lookup and removal.
- **SendMessageBatch10**: ~1.5 µs for 10 messages (~147 ns/message) — batch overhead is minimal.
- **ConcurrentSendReceive**: ~345 ns — demonstrates good concurrency with mutex-based access.

## SQLite Store Baseline Results

| Benchmark | Time/op | Memory/op | Allocs/op |
|-----------|---------|-----------|-----------|
| `BenchmarkSQLite_SendMessage` | 614.6 µs | 2,306 B | 39 |
| `BenchmarkSQLite_ReceiveMessage` | 344.2 µs | 6,764 B | 118 |
| `BenchmarkSQLite_DeleteMessage` | 244.1 µs | 1,051 B | 24 |
| `BenchmarkSQLite_SendMessageBatch10` | 3,199.2 µs | 22,957 B | 397 |
| `BenchmarkSQLite_ConcurrentSendReceive` | 651.3 µs | 8,313 B | 148 |

### Key Observations

- **SendMessage**: ~615 µs — dominated by SQLite INSERT with WAL mode.
- **ReceiveMessage**: ~344 µs — includes SELECT, visibility timeout update, and receipt handle generation.
- **DeleteMessage**: ~244 µs — DELETE operation with index lookup.
- **SendMessageBatch10**: ~3.2 ms for 10 messages (~320 µs/message) — per-message overhead is higher than single sends due to transaction management.
- **ConcurrentSendReceive**: ~651 µs — SQLite's write locking adds contention under concurrency.

### Memory vs SQLite Comparison

| Operation | Memory | SQLite | SQLite Overhead |
|-----------|--------|--------|-----------------|
| SendMessage | 310 ns | 615 µs | ~1,984× |
| ReceiveMessage | 2.2 µs | 344 µs | ~156× |
| DeleteMessage | 36 ns | 244 µs | ~6,778× |

The memory store is orders of magnitude faster, as expected. SQLite provides durability at the cost of disk I/O latency.

## Handler Pipeline Baseline Results

| Benchmark | Time/op | Memory/op | Allocs/op |
|-----------|---------|-----------|-----------|
| `BenchmarkHandleRequest_SendMessage` | 770.0 ns | 1,077 B | 8 |
| `BenchmarkHandleRequest_ReceiveMessage` | 3,171 ns | 3,650 B | 29 |

### Key Observations

- **SendMessage**: ~770 ns — includes protocol parsing, handler dispatch, and memory store write.
- **ReceiveMessage**: ~3.2 µs — includes protocol parsing, handler dispatch, and memory store read with visibility timeout management.

The handler pipeline adds ~460 ns overhead on top of the raw store operation (770 ns - 310 ns for send, 3,171 ns - 2,208 ns for receive), which is the cost of protocol parsing and HTTP request handling.

## Notes

- Benchmarks use `100x` (fixed iteration count) instead of time-based (`1s`) to avoid excessive pre-population times for DeleteMessage benchmarks.
- `ReceiveMessages` returns a maximum of 10 messages per call — benchmarks account for this by receiving in batches.
- DeleteMessage benchmarks use a large visibility timeout (3600s) to prevent receipt handles from expiring during the benchmark.
- All benchmarks use `b.ResetTimer()` and `b.ReportAllocs()` for accurate measurements.
- BadgerDB store benchmarks are not yet available. The BadgerStore uses lazy visibility timeout evaluation (similar to SQLiteStore), so performance characteristics are expected to be comparable to SQLite with some variation due to BadgerDB's LSM tree architecture.
