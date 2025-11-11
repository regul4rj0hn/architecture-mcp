---
inclusion: fileMatch
fileMatchPattern: '*_test.go'
---

# Testing Standards & Practices

## Philosophy

Test behavior, not implementation. Focus on critical paths and error handling. Avoid over-testing obvious code.

## Coverage Targets

- `internal/server/` - 75%+
- `pkg/cache/`, `pkg/prompts/` - 80%+
- `pkg/scanner/`, `pkg/monitor/` - 70%+
- `internal/models/` - 60%+
- `pkg/errors/`, `pkg/logging/` - 70%+

Don't test: getters/setters, stdlib, third-party libs, generated code, trivial constructors

## Patterns

**Table-driven tests** for multiple scenarios:
```go
tests := []struct {
    name string
    input interface{}
    want interface{}
    wantErr bool
}{
    {"valid case", validInput, expected, false},
    {"error case", badInput, nil, true},
}
for _, tt := range tests {
    t.Run(tt.name, func(t *testing.T) {
        got, err := function(tt.input)
        if (err != nil) != tt.wantErr {
            t.Errorf("error = %v, wantErr %v", err, tt.wantErr)
        }
        if got != tt.want {
            t.Errorf("got %v, want %v", got, tt.want)
        }
    })
}
```

**Filesystem tests** - use `t.TempDir()` for auto-cleanup:
```go
tmpDir := t.TempDir()
testFile := filepath.Join(tmpDir, "test.md")
os.WriteFile(testFile, []byte("# Test"), 0644)
```

**Benchmarks** - reset timer after setup:
```go
func BenchmarkOperation(b *testing.B) {
    setup := prepareData()
    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        operation(setup)
    }
}
```

## Organization

**File structure**: Tests alongside source. Split files >800 lines by functional area.

**Naming**: `Test<Function>_<Scenario>` (e.g., `TestScanDirectory_EmptyDirectory`)

**Fixtures**: Inline for small data, helper functions for reusable, `testdata/` for large files

**Helpers**: Use `t.Helper()` to report caller's line in errors

## Key Rules

- Use explicit checks, not assertion libraries: `if got != want { t.Errorf(...) }`
- Test concurrent access with goroutines + `sync.WaitGroup`
- Run race detector: `go test -race ./...`
- Tests must be independent (no shared state, order doesn't matter)
- Use `t.Cleanup()` or `defer` for resource cleanup
- Split test files >800 lines by functional area
- Fix failing tests immediately (no "flaky tests")

## Commands

```bash
make test              # Run all tests
make test-coverage     # Coverage report
go test -race ./...    # Race detector
go test -run TestName  # Specific test
```
