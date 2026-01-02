# Concurrency and Parallel Execution in gosible

## Overview

gosible supports parallel execution of tasks across multiple hosts, providing significant performance improvements over serial execution. This document explains how concurrency works and how to configure it.

## Default Concurrency Settings

### Default Values

- **Default max concurrency**: `5` parallel hosts
- **Connection TTL**: `30` minutes
- **Task execution**: Parallel by default

### Why the Default is 5

The default concurrency limit of 5 is chosen to:

1. **Prevent resource exhaustion** on the control machine
2. **Avoid overwhelming target hosts** with too many simultaneous connections
3. **Maintain network stability** by limiting concurrent SSH connections
4. **Balance performance** with system resource usage

## Configuring Concurrency

### Method 1: Using TaskRunner

```go
import "github.com/liliang-cn/gosible/pkg/runner"

taskRunner := runner.NewTaskRunner()
taskRunner.SetMaxConcurrency(10) // Execute on up to 10 hosts in parallel
```

### Method 2: Custom TaskRunner

```go
taskRunner := runner.NewTaskRunnerWithDependencies(
    moduleRegistry,
    connectionMgr,
    varMgr,
)
taskRunner.SetMaxConcurrency(20)
```

## Performance Considerations

### When to Increase Concurrency

- **Large target environments** (100+ hosts)
- **Fast network connections** with low latency
- **Powerful control machine** with ample CPU/memory
- **Long-running tasks** where parallel execution saves significant time

Example:
```go
// For large-scale deployments
taskRunner.SetMaxConcurrency(50)
```

### When to Decrease Concurrency

- **Resource-constrained targets** (embedded systems, small VMs)
- **Network-limited environments** with high latency
- **Memory-intensive tasks** on the control machine
- **Rate-limited APIs** or services

Example:
```go
// For resource-constrained environments
taskRunner.SetMaxConcurrency(2)
```

## Benchmarks

Based on testing with Ubuntu 22.04 hosts over SSH:

| Concurrency | 100 Tasks | Time | Throughput |
|-------------|-----------|------|------------|
| 1 (serial)  | 300 ops   | ~10s | 30 ops/sec |
| 5 (default)| 300 ops   | ~3s  | 96 ops/sec |
| 10          | 300 ops   | ~2s  | 150 ops/sec |
| 20          | 300 ops   | ~1.5s| 200 ops/sec |

**Note**: Actual performance varies based on:
- Network latency and bandwidth
- Task complexity and duration
- Target host resource availability
- Control machine specifications

## Connection Pooling

gosible implements automatic connection pooling to maximize efficiency:

### How It Works

1. **First connection**: Establishes SSH connection
2. **Subsequent tasks**: Reuses existing connections
3. **Connection TTL**: Connections cached for 30 minutes
4. **Automatic cleanup**: Idle connections closed after TTL

### Benefits

- **Reduced overhead**: No repeated SSH handshakes
- **Faster execution**: Skip authentication on cached connections
- **Resource efficiency**: Fewer simultaneous TCP connections

### Example Impact

```go
// First execution: ~100ms (includes SSH handshake)
// Subsequent executions: ~10ms (connection reused)
for i := 0; i < 100; i++ {
    taskRunner.ExecuteTask(ctx, "test", "command", 
        map[string]interface{}{"cmd": "echo test"}, hosts, nil)
}
// Total: ~2s with connection pooling vs ~10s without
```

## Thread Safety

The TaskRunner is **thread-safe** and supports concurrent goroutines:

```go
var wg sync.WaitGroup

// Launch 10 goroutines executing tasks in parallel
for i := 0; i < 10; i++ {
    wg.Add(1)
    go func(id int) {
        defer wg.Done()
        
        taskRunner.ExecuteTask(ctx, fmt.Sprintf("Task %d", id), 
            "command", map[string]interface{}{"cmd": "hostname"}, hosts, nil)
    }(i)
}

wg.Wait()
```

### Concurrency Limit Considerations

When using goroutines:
- **Each goroutine** may execute tasks on up to `max_concurrency` hosts
- **Total parallelism** = `goroutines` × `max_concurrency`
- **Adjust max_concurrency** based on your goroutine count

Example:
```go
// 10 goroutines × 5 concurrency = 50 potential parallel connections
taskRunner.SetMaxConcurrency(5) // Safe for 10 goroutines

// 10 goroutines × 20 concurrency = 200 potential parallel connections
// May overwhelm targets or control machine
taskRunner.SetMaxConcurrency(20)
```

## Best Practices

### 1. Start with Defaults

```go
// Default (5) works well for most use cases
taskRunner := runner.NewTaskRunner()
```

### 2. Scale Gradually

```go
// Increase incrementally and monitor performance
taskRunner.SetMaxConcurrency(5)   // Start here
taskRunner.SetMaxConcurrency(10)  // Increase if needed
taskRunner.SetMaxConcurrency(20)  // Increase more if still beneficial
```

### 3. Monitor Resources

```go
// Watch for:
// - Control machine: CPU, memory, file descriptors
// - Target hosts: CPU, load average, SSH processes
// - Network: Bandwidth, connection count
```

### 4. Consider Task Characteristics

```go
// Quick tasks (sub-second): Higher concurrency beneficial
taskRunner.SetMaxConcurrency(20)

// Long tasks (minutes+): Lower concurrency usually sufficient
taskRunner.SetMaxConcurrency(5)

// Resource-intensive tasks: Lower concurrency to avoid overload
taskRunner.SetMaxConcurrency(2)
```

### 5. Test in Your Environment

```go
// Benchmark to find optimal concurrency for your setup
start := time.Now()
for concurrency := 1; concurrency <= 20; concurrency++ {
    taskRunner.SetMaxConcurrency(concurrency)
    
    // Run test workload
    for i := 0; i < 100; i++ {
        taskRunner.ExecuteTask(ctx, "test", "command", 
            map[string]interface{}{"cmd": "echo test"}, hosts, nil)
    }
    
    duration := time.Since(start)
    fmt.Printf("Concurrency %d: %v\n", concurrency, duration)
}
```

## Troubleshooting

### Issue: "Too many open files"

**Cause**: Exceeded file descriptor limit

**Solutions**:
```bash
# Increase ulimit on control machine
ulimit -n 4096

# Or reduce concurrency
taskRunner.SetMaxConcurrency(3)
```

### Issue: High memory usage

**Cause**: Too many concurrent connections/outputs

**Solutions**:
```go
// Reduce concurrency
taskRunner.SetMaxConcurrency(3)

// Reduce connection TTL
// (requires custom TaskRunner initialization)

// Stream outputs instead of buffering
// (see examples/streaming_example.go)
```

### Issue: Target hosts overwhelmed

**Symptoms**: Timeouts, high load average, slow response

**Solutions**:
```go
// Reduce concurrency
taskRunner.SetMaxConcurrency(2)

// Add delays between tasks (custom implementation)
// Or use rate limiting
```

## Advanced Configuration

### Custom Connection Manager

```go
import "github.com/liliang-cn/gosible/pkg/connection"

connMgr := connection.NewConnectionManager()
connMgr.SetConnectionTTL(10 * time.Minute) // Shorter TTL
connMgr.SetMaxConnections(50)              // Connection pool size

taskRunner := runner.NewTaskRunnerWithDependencies(
    modules.DefaultModuleRegistry,
    connMgr,
    vars.NewVarManager(),
)
```

### Per-Task Concurrency Control

```go
// Serial execution (1 host at a time)
taskRunner.SetMaxConcurrency(1)
results, _ := taskRunner.ExecuteTask(ctx, "task1", "command", args1, hosts, vars)

// Parallel execution (5 hosts at a time)
taskRunner.SetMaxConcurrency(5)
results, _ = taskRunner.ExecuteTask(ctx, "task2", "command", args2, hosts, vars)

// High concurrency (20 hosts at a time)
taskRunner.SetMaxConcurrency(20)
results, _ = taskRunner.ExecuteTask(ctx, "task3", "command", args3, hosts, vars)
```

## See Also

- [Playbook Execution](docs/PLAYBOOKS.md) - Concurrency in playbook context
- [Connection Management](docs/CONNECTIONS.md) - Connection pooling details
- [Performance Tuning](docs/PERFORMANCE.md) - Complete performance guide
- [Examples](../examples/) - Real-world concurrency examples

## Summary

- **Default concurrency**: 5 parallel hosts
- **Configure via**: `taskRunner.SetMaxConcurrency(n)`
- **Connection pooling**: Automatic, 30-minute TTL
- **Thread-safe**: Yes, supports goroutines
- **Optimal value**: Depends on your environment and workload
- **Recommendation**: Start with default, scale based on benchmarks

For questions or issues, please open a GitHub issue.
