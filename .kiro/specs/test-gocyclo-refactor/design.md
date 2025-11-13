# Design Document

## Overview

This design outlines the approach to improve the Go Report Card score by adding a LICENSE file and refactoring test functions with high cyclomatic complexity. The goal is to reduce complexity in test functions from values ranging 16-46 down to below 15, while maintaining identical test coverage and behavior.

## Architecture

### License Addition

The LICENSE file will be added to the project root directory with standard MIT License text. This is a straightforward file addition with no architectural implications.

### Test Refactoring Strategy

The refactoring will follow a consistent pattern across all affected test files:

1. **Extract Helper Functions**: Break down large test functions into smaller, focused helper functions
2. **Table-Driven Tests**: Convert repetitive test logic into table-driven patterns where appropriate
3. **Sub-Test Extraction**: Move complex sub-tests into separate test functions
4. **Validation Helpers**: Create reusable validation functions for common assertion patterns

## Components and Interfaces

### Helper Function Patterns

**Validation Helpers**
- Purpose: Encapsulate common assertion logic
- Naming: `validate<Aspect>(t *testing.T, ...)`
- Example: `validateResourceStructure(t *testing.T, resource models.MCPResource)`

**Setup Helpers**
- Purpose: Prepare test fixtures and environment
- Naming: `setup<Context>(t *testing.T) <ReturnType>`
- Example: `setupTestDocuments(t *testing.T, tempDir string) map[string]string`

**Execution Helpers**
- Purpose: Execute specific test scenarios
- Naming: `test<Scenario>(t *testing.T, ...)`
- Example: `testFileCreation(t *testing.T, monitor *FileSystemMonitor, tempDir string)`

### Refactoring Approach by File

#### internal/server/integration_test.go

**TestMCPResourceMethodsIntegration (complexity 46)**
- Extract document setup into `setupTestDocuments()`
- Extract directory creation into `setupTestDirectories()`
- Split into separate test functions:
  - `TestResourcesListIntegration`
  - `TestResourcesReadGuidelinesIntegration`
  - `TestResourcesReadPatternsIntegration`
  - `TestResourcesReadADRIntegration`
- Create validation helpers:
  - `validateResourceListResponse()`
  - `validateResourceReadResponse()`
  - `validateResourceContent()`

**TestMCPProtocolComplianceIntegration (complexity 32)**
- Extract into separate test functions:
  - `TestJSONRPCCompliance`
  - `TestMCPResourceStructureCompliance`
  - `TestMCPResourceContentCompliance`
  - `TestErrorResponseCompliance`
- Create validation helper: `validateJSONRPCResponse()`

**TestResourceContentRetrievalIntegration (complexity 20)**
- Extract test document creation into `setupContentTestDocuments()`
- Split content verification into helper: `validateContentText()`
- Separate test scenarios into distinct sub-tests

**TestDocumentationSystemIntegration (complexity 19)**
- Extract directory setup into `setupDocumentationDirectories()`
- Extract file operations into helpers:
  - `testFileModification()`
  - `testFileDeletion()`
- Create cache validation helper: `validateCacheState()`

**TestMCPResourceErrorScenariosIntegration (complexity 16)**
- Convert to table-driven test with error scenarios
- Create helper: `testErrorScenario()`

#### pkg/monitor/monitor_test.go

**TestFileSystemMonitorIntegration (complexity 20)**
- Extract event collection setup into `setupEventCollection()`
- Split file operations into separate helpers:
  - `testFileCreationEvent()`
  - `testFileModificationEvent()`
  - `testFileDeletionEvent()`
- Create event validation helper: `validateFileEvent()`

#### pkg/errors/circuit_breaker_test.go

**TestCircuitBreaker (complexity 18)**
- Split into separate test functions:
  - `TestCircuitBreakerInitialState`
  - `TestCircuitBreakerOpensAfterFailures`
  - `TestCircuitBreakerRejectsWhenOpen`
  - `TestCircuitBreakerTransitionsToHalfOpen`
  - `TestCircuitBreakerClosesAfterSuccess`
  - `TestCircuitBreakerReopensOnFailure`
- Create helper: `executeFailingOperations()`

#### internal/server/server_resources_test.go

**TestHandleResourcesList (complexity 18)**
- Extract document setup into `setupTestCacheDocuments()`
- Create validation helpers:
  - `validateResourceListBasics()`
  - `validateResourceProperties()`
  - `validateResourceURIs()`

#### internal/server/server_prompts_test.go

**TestHandlePromptsGet (complexity 16)**
- Extract prompt setup into `setupTestPrompt()`
- Create validation helpers:
  - `validatePromptsGetResponse()`
  - `validateMessageStructure()`
  - `validateArgumentSubstitution()`

#### pkg/errors/graceful_degradation_test.go

**TestGracefulDegradationManager (complexity 27)**
- Split into separate test functions:
  - `TestGracefulDegradationRegistration`
  - `TestGracefulDegradationErrorRecording`
  - `TestGracefulDegradationTimeWindow`
  - `TestGracefulDegradationRecovery`
  - `TestGracefulDegradationOverallHealth`
  - `TestGracefulDegradationExecution`
  - `TestGracefulDegradationForceRecovery`
- Create helper: `setupDegradationManager()`

#### pkg/logging/manager_test.go

**TestLoggingManagerSpecializedMethods (complexity 18)**
- Split into separate test functions for each specialized method
- Create validation helper: `validateLogOutput()`

#### pkg/scanner/scanner_test.go

**TestScanDirectoryIntegration (complexity 17)**
- Extract directory setup into `setupScanTestDirectories()`
- Create validation helper: `validateScannedDocuments()`
- Split verification logic into focused helpers

## Data Models

No changes to data models are required. All refactoring maintains existing interfaces and structures.

## Error Handling

The refactoring will preserve all existing error handling behavior:
- Test failures will continue to use `t.Error()` and `t.Fatal()` appropriately
- Helper functions will accept `*testing.T` to report errors directly
- No changes to production error handling code

## Testing Strategy

### Validation Approach

1. **Before Refactoring**: Run full test suite and capture coverage metrics
   ```bash
   make test
   make test-coverage
   ```

2. **During Refactoring**: Run tests after each file refactoring
   ```bash
   go test ./internal/server/... -v
   go test ./pkg/monitor/... -v
   go test ./pkg/errors/... -v
   ```

3. **After Refactoring**: Verify improvements
   ```bash
   # Check cyclomatic complexity
   gocyclo -over 15 .
   
   # Verify all tests pass
   make test
   
   # Verify coverage unchanged
   make test-coverage
   ```

### Success Criteria

- All tests pass with identical behavior
- No reduction in test coverage
- All test functions have cyclomatic complexity ≤ 15
- Code remains idiomatic and readable
- Helper functions have clear, descriptive names

## Implementation Principles

### Code Quality

- **Single Responsibility**: Each helper function should have one clear purpose
- **Descriptive Naming**: Function names should clearly indicate what they test or validate
- **Minimal Duplication**: Extract common patterns into reusable helpers
- **Readability**: Refactored code should be easier to understand than original

### Testing Best Practices

- **Preserve Test Intent**: Maintain the original test's purpose and assertions
- **Keep Context**: Don't lose important test context in extraction
- **Fail Fast**: Use `t.Fatal()` for setup failures, `t.Error()` for assertion failures
- **Clear Messages**: Maintain descriptive error messages in assertions

### Refactoring Safety

- **One File at a Time**: Complete and verify each file before moving to the next
- **Run Tests Frequently**: Verify tests pass after each significant change
- **Preserve Coverage**: Ensure no test scenarios are lost during refactoring
- **No Behavior Changes**: Tests should verify exactly the same conditions as before

## Refactoring Order

The refactoring will proceed in this order to minimize risk:

1. **LICENSE file** - Simple addition, no risk
2. **pkg/errors/circuit_breaker_test.go** - Moderate complexity, clear separation
3. **pkg/monitor/monitor_test.go** - Single complex function
4. **internal/server/server_resources_test.go** - Moderate complexity
5. **internal/server/server_prompts_test.go** - Moderate complexity
6. **pkg/scanner/scanner_test.go** - Moderate complexity
7. **pkg/logging/manager_test.go** - Moderate complexity
8. **pkg/errors/graceful_degradation_test.go** - High complexity
9. **internal/server/integration_test.go** - Highest complexity, most critical

This order allows us to build confidence with simpler refactorings before tackling the most complex integration tests.

## Race Condition Fixes

### Overview

The race detector has identified several data race conditions in concurrent code. These must be fixed to ensure thread-safe operation and reliable test execution in CI environments.

### Identified Race Conditions

#### 1. Cache System (pkg/cache/cache.go)

**Issue**: The `Get()` method updates the `lastAccessed` map and stats without proper synchronization.

**Location**: Lines 107, 113 in cache.go

**Root Cause**: 
- The `lastAccessed` map is accessed without holding the write lock
- Stats updates occur outside the mutex protection

**Solution**:
- Move `lastAccessed` map updates inside the write lock section
- Ensure stats updates are protected by the mutex
- Use `defer` to ensure lock is always released

**Implementation**:
```go
func (c *DocumentCache) Get(uri string) (*models.Document, bool) {
    c.mu.RLock()
    doc, exists := c.documents[uri]
    c.mu.RUnlock()
    
    if exists {
        c.mu.Lock()
        c.lastAccessed[uri] = time.Now()
        c.stats.Hits++
        c.mu.Unlock()
    } else {
        c.mu.Lock()
        c.stats.Misses++
        c.mu.Unlock()
    }
    
    return doc, exists
}
```

#### 2. Monitor System (pkg/monitor/monitor.go)

**Issue**: The `debounceTimers` map is accessed without proper synchronization when deleting entries.

**Location**: Line 92 in monitor.go

**Root Cause**:
- Map deletion occurs in a goroutine without mutex protection
- Multiple goroutines can access the map concurrently

**Solution**:
- Add mutex protection around all `debounceTimers` map operations
- Ensure the mutex is held during map reads, writes, and deletes

**Implementation**:
```go
// In monitorEvents function, protect map access:
m.mu.Lock()
delete(m.debounceTimers, event.Name)
m.mu.Unlock()
```

#### 3. Error System - Circuit Breaker (pkg/errors/circuit_breaker_test.go)

**Issue**: Test code accesses callback state variables without synchronization.

**Location**: Lines 265, 268, 271 in circuit_breaker_test.go

**Root Cause**:
- Callback variables are accessed from both the main test goroutine and callback goroutine
- No synchronization mechanism protects these accesses

**Solution**:
- Use `sync.Mutex` to protect callback state variables
- Lock before reading/writing callback state in tests

**Implementation**:
```go
var (
    callbackMu sync.Mutex
    callbackInvoked bool
    oldState CircuitState
    newState CircuitState
)

// In callback:
callbackMu.Lock()
callbackInvoked = true
oldState = old
newState = new
callbackMu.Unlock()

// In test assertions:
callbackMu.Lock()
if !callbackInvoked {
    t.Error("callback not invoked")
}
callbackMu.Unlock()
```

#### 4. Error System - Graceful Degradation (pkg/errors/graceful_degradation_test.go)

**Issue**: Test code accesses callback state variables without synchronization.

**Location**: Lines 302-305, 323, 326, 329, 332 in graceful_degradation_test.go

**Root Cause**:
- Similar to circuit breaker, callback variables accessed without protection
- Multiple goroutines access shared state

**Solution**:
- Use `sync.Mutex` to protect callback state variables
- Same pattern as circuit breaker fix

#### 5. Prompts System (pkg/prompts/manager_test.go)

**Issue**: Test code accesses reload state without synchronization.

**Location**: Lines 551 in manager_test.go

**Root Cause**:
- File event handling triggers reload in a separate goroutine
- Test checks reload state without synchronization

**Solution**:
- Use `sync.Mutex` or atomic operations to protect reload state
- Add proper synchronization in test assertions

**Implementation**:
```go
var (
    reloadMu sync.Mutex
    reloadTriggered bool
)

// In file event handler:
reloadMu.Lock()
reloadTriggered = true
reloadMu.Unlock()

// In test:
time.Sleep(600 * time.Millisecond) // Wait for debounce
reloadMu.Lock()
triggered := reloadTriggered
reloadMu.Unlock()

if !triggered {
    t.Error("reload not triggered")
}
```

#### 6. Integration Tests (internal/server/integration_test.go)

**Issue**: Cache operations in integration tests trigger race conditions.

**Location**: Lines 104, 123 in integration_test.go

**Root Cause**:
- Integration test accesses cache while file monitor goroutines also access it
- No synchronization between test and background operations

**Solution**:
- Ensure proper synchronization when verifying cache state
- Add delays or explicit synchronization points
- Fix underlying cache race conditions (see #1 above)

### Testing Strategy for Race Fixes

1. **Verify Each Fix**: Run `go test -race -short ./...` after each fix
2. **Isolate Tests**: Test individual packages to isolate race conditions
3. **CI Verification**: Ensure GitHub Actions passes with race detector enabled

### Implementation Order for Race Fixes

1. **Cache System** - Most critical, affects multiple tests
2. **Monitor System** - Affects file watching tests
3. **Error System Tests** - Test-only fixes, lower risk
4. **Prompts System Tests** - Test-only fixes, lower risk
5. **Integration Tests** - Depends on cache fix, test last
