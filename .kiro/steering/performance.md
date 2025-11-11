---
inclusion: fileMatch
fileMatchPattern: '(server|cache|scanner|monitor|prompts)\.go'
---

# Performance Requirements & Optimization

## Philosophy

Clarity first, performance second. Measure before optimizing. Focus on algorithms over micro-optimizations.

## SLAs

**MCP Operations**: initialize <50ms, resources/list <100ms, resources/read <50ms cached/<200ms disk, prompts/list <100ms, prompts/get <500ms

**System**: Cache population <5s/1000 docs, file change detection <2s, prompt reload <500ms, shutdown <3s

## Limits

**Memory**: 256MB container, <1MB per document, <5MB prompt rendering
**CPU**: 0.2 CPU (20% of one core), worker pools capped at min(NumCPU, 8)
**I/O**: Cache aggressively, batch operations, use buffered I/O

## Patterns

**Worker pools** for I/O: Cap at min(NumCPU, 8), scale by file count (2 for ≤10, 4 for ≤100, 8 for >100)

**Concurrent init**: Start scanner, prompts, monitor in parallel goroutines, collect with timeout

**Caching**:
- Documents: Metadata on startup, content lazy-loaded, invalidate on fs events, no TTL
- Prompts: Full reload on change, debounce 500ms
- Resources: Cache by URI during rendering, track hit rate

**RWMutex**: Use for read-heavy workloads (cache lookups). RLock for reads, Lock for writes.

## Optimization

**Reduce allocations**: Use `strings.Builder` for string concatenation, preallocate slices with capacity

**Lock scope**: Hold locks only during data access, not during expensive operations. Release before processing.

**I/O**: Use `bufio.Reader` for large files, batch operations with worker pools

## When to Optimize

Optimize when: profiling shows bottleneck, exceeds SLA, approaches resource limits
Don't optimize: no measured problem, already fast enough, sacrifices clarity

**Workflow**: Measure → Hypothesize → Implement → Benchmark → Review → Document WHY

## Monitoring

**Track**: Cache hit rate (>90%), P50/P95/P99 latency, memory/CPU usage, goroutine count, error rates

**Benchmark**: `go test -bench=. -benchmem -benchtime=5s ./...`

**Profile**: `go test -bench=Name -cpuprofile=cpu.prof && go tool pprof cpu.prof`

**CI**: Benchmarks on every PR, fail on >10% regression, race detector must pass
