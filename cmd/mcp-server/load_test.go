package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"mcp-architecture-service/internal/models"
	"mcp-architecture-service/internal/server"
)

// LoadTestConfig defines configuration for load testing
type LoadTestConfig struct {
	Duration          time.Duration
	ConcurrentClients int
	RequestsPerSecond int
	DocumentCount     int
	DocumentSize      int
}

// LoadTestResults contains the results of a load test
type LoadTestResults struct {
	TotalRequests     int64
	SuccessfulReqs    int64
	FailedReqs        int64
	AvgResponseTime   time.Duration
	MinResponseTime   time.Duration
	MaxResponseTime   time.Duration
	P95ResponseTime   time.Duration
	P99ResponseTime   time.Duration
	RequestsPerSecond float64
	ErrorRate         float64
	MemoryUsage       runtime.MemStats
}

// TestLoadTestFullSystem performs comprehensive load testing of the entire MCP system
func TestLoadTestFullSystem(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping load test in short mode")
	}

	testCases := []struct {
		name   string
		config LoadTestConfig
	}{
		{
			name: "Light_Load",
			config: LoadTestConfig{
				Duration:          30 * time.Second,
				ConcurrentClients: 5,
				RequestsPerSecond: 10,
				DocumentCount:     100,
				DocumentSize:      1024,
			},
		},
		{
			name: "Medium_Load",
			config: LoadTestConfig{
				Duration:          60 * time.Second,
				ConcurrentClients: 20,
				RequestsPerSecond: 50,
				DocumentCount:     1000,
				DocumentSize:      5120,
			},
		},
		{
			name: "Heavy_Load",
			config: LoadTestConfig{
				Duration:          120 * time.Second,
				ConcurrentClients: 50,
				RequestsPerSecond: 100,
				DocumentCount:     5000,
				DocumentSize:      10240,
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			results := runLoadTest(t, tc.config)
			validateLoadTestResults(t, tc.config, results)
			logLoadTestResults(t, tc.name, results)
		})
	}
}

// TestLoadTestMemoryPressure tests system behavior under memory pressure
func TestLoadTestMemoryPressure(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping memory pressure test in short mode")
	}

	config := LoadTestConfig{
		Duration:          60 * time.Second,
		ConcurrentClients: 30,
		RequestsPerSecond: 75,
		DocumentCount:     10000, // Large number of documents
		DocumentSize:      51200, // 50KB per document
	}

	t.Log("Starting memory pressure test...")
	results := runLoadTest(t, config)

	// Check memory usage
	memoryUsageMB := float64(results.MemoryUsage.Alloc) / 1024 / 1024
	t.Logf("Peak memory usage: %.2f MB", memoryUsageMB)

	// Validate that system remained stable under memory pressure
	if results.ErrorRate > 5.0 {
		t.Errorf("Error rate too high under memory pressure: %.2f%%", results.ErrorRate)
	}

	if results.P95ResponseTime > 500*time.Millisecond {
		t.Errorf("P95 response time too high under memory pressure: %v", results.P95ResponseTime)
	}

	logLoadTestResults(t, "Memory_Pressure", results)
}

// TestLoadTestSustainedLoad tests system behavior under sustained load
func TestLoadTestSustainedLoad(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping sustained load test in short mode")
	}

	config := LoadTestConfig{
		Duration:          300 * time.Second, // 5 minutes
		ConcurrentClients: 25,
		RequestsPerSecond: 40,
		DocumentCount:     2000,
		DocumentSize:      2048,
	}

	t.Log("Starting sustained load test (5 minutes)...")
	results := runLoadTest(t, config)

	// Validate sustained performance
	if results.ErrorRate > 1.0 {
		t.Errorf("Error rate too high during sustained load: %.2f%%", results.ErrorRate)
	}

	expectedRPS := float64(config.RequestsPerSecond * config.ConcurrentClients)
	if results.RequestsPerSecond < expectedRPS*0.8 {
		t.Errorf("Throughput degraded during sustained load: got %.2f RPS, expected at least %.2f RPS",
			results.RequestsPerSecond, expectedRPS*0.8)
	}

	logLoadTestResults(t, "Sustained_Load", results)
}

// TestLoadTestBurstTraffic tests system behavior under burst traffic patterns
func TestLoadTestBurstTraffic(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping burst traffic test in short mode")
	}

	// Create server with test documents
	mcpServer, cleanup := setupLoadTestServer(t, 1000, 2048)
	defer cleanup()

	t.Log("Starting burst traffic test...")

	var totalRequests int64
	var successfulReqs int64
	var failedReqs int64
	var responseTimes []time.Duration
	var responseTimesMutex sync.Mutex

	// Simulate burst pattern: 10 seconds high load, 5 seconds low load, repeat
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	var wg sync.WaitGroup

	// Burst worker
	burstWorker := func(highLoad bool) {
		clients := 50
		if !highLoad {
			clients = 5
		}

		for i := 0; i < clients; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for {
					select {
					case <-ctx.Done():
						return
					default:
						start := time.Now()
						success := performMCPRequest(mcpServer)
						responseTime := time.Since(start)

						atomic.AddInt64(&totalRequests, 1)
						if success {
							atomic.AddInt64(&successfulReqs, 1)
						} else {
							atomic.AddInt64(&failedReqs, 1)
						}

						responseTimesMutex.Lock()
						responseTimes = append(responseTimes, responseTime)
						responseTimesMutex.Unlock()

						time.Sleep(20 * time.Millisecond) // Rate limiting
					}
				}
			}()
		}
	}

	// Run burst pattern
	go func() {
		ticker := time.NewTicker(15 * time.Second)
		defer ticker.Stop()

		highLoad := true
		burstWorker(highLoad)

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				highLoad = !highLoad
				t.Logf("Switching to %s load", map[bool]string{true: "high", false: "low"}[highLoad])
				burstWorker(highLoad)
			}
		}
	}()

	<-ctx.Done()
	wg.Wait()

	// Calculate results
	results := calculateLoadTestResults(totalRequests, successfulReqs, failedReqs, responseTimes, 60*time.Second)

	// Validate burst handling
	if results.ErrorRate > 2.0 {
		t.Errorf("Error rate too high during burst traffic: %.2f%%", results.ErrorRate)
	}

	logLoadTestResults(t, "Burst_Traffic", results)
}

// runLoadTest executes a load test with the given configuration
func runLoadTest(t *testing.T, config LoadTestConfig) LoadTestResults {
	// Setup server with test documents
	mcpServer, cleanup := setupLoadTestServer(t, config.DocumentCount, config.DocumentSize)
	defer cleanup()

	var totalRequests int64
	var successfulReqs int64
	var failedReqs int64
	var responseTimes []time.Duration
	var responseTimesMutex sync.Mutex

	ctx, cancel := context.WithTimeout(context.Background(), config.Duration)
	defer cancel()

	var wg sync.WaitGroup

	// Start concurrent clients
	for i := 0; i < config.ConcurrentClients; i++ {
		wg.Add(1)
		go func(clientID int) {
			defer wg.Done()

			// Calculate delay between requests for this client
			requestInterval := time.Duration(int64(time.Second) / int64(config.RequestsPerSecond))

			ticker := time.NewTicker(requestInterval)
			defer ticker.Stop()

			for {
				select {
				case <-ctx.Done():
					return
				case <-ticker.C:
					start := time.Now()
					success := performMCPRequest(mcpServer)
					responseTime := time.Since(start)

					atomic.AddInt64(&totalRequests, 1)
					if success {
						atomic.AddInt64(&successfulReqs, 1)
					} else {
						atomic.AddInt64(&failedReqs, 1)
					}

					responseTimesMutex.Lock()
					responseTimes = append(responseTimes, responseTime)
					responseTimesMutex.Unlock()
				}
			}
		}(i)
	}

	wg.Wait()

	return calculateLoadTestResults(totalRequests, successfulReqs, failedReqs, responseTimes, config.Duration)
}

// setupLoadTestServer creates and initializes a server for load testing
func setupLoadTestServer(t *testing.T, docCount, docSize int) (*server.MCPServer, func()) {
	// Create temporary directory for test documents
	tempDir, err := os.MkdirTemp("", "load_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}

	// Create test documentation structure
	createLoadTestDocuments(t, tempDir, docCount, docSize)

	// Create and initialize server
	mcpServer := server.NewMCPServer()

	// Change to temp directory for initialization
	originalDir, _ := os.Getwd()
	os.Chdir(tempDir)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := mcpServer.Start(ctx); err != nil && err != context.DeadlineExceeded {
		os.Chdir(originalDir)
		os.RemoveAll(tempDir)
		t.Fatalf("Failed to start server: %v", err)
	}

	cleanup := func() {
		os.Chdir(originalDir)
		mcpServer.Shutdown(context.Background())
		os.RemoveAll(tempDir)
	}

	return mcpServer, cleanup
}

// createLoadTestDocuments creates test documentation files
func createLoadTestDocuments(t *testing.T, baseDir string, count, size int) {
	categories := []struct {
		name string
		dir  string
	}{
		{"guideline", "docs/guidelines"},
		{"pattern", "docs/patterns"},
		{"adr", "docs/adr"},
	}

	// Create directories
	for _, cat := range categories {
		dir := filepath.Join(baseDir, cat.dir)
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatalf("Failed to create directory %s: %v", dir, err)
		}
	}

	// Create content template
	contentTemplate := strings.Repeat("This is test content for load testing. ", size/50)

	// Create documents
	for i := 0; i < count; i++ {
		cat := categories[i%len(categories)]

		content := fmt.Sprintf("# Load Test %s %d\n\n%s\n\n## Details\n\nDocument %d for load testing the MCP architecture service.\n\n%s",
			cat.name, i, contentTemplate, i, contentTemplate)

		filename := fmt.Sprintf("load-test-%d.md", i)
		filePath := filepath.Join(baseDir, cat.dir, filename)

		if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
			t.Fatalf("Failed to create file %s: %v", filePath, err)
		}
	}
}

// performMCPRequest performs a single MCP request and returns success status
func performMCPRequest(mcpServer *server.MCPServer) bool {
	// Alternate between different types of requests
	requestType := time.Now().UnixNano() % 3

	var message *models.MCPMessage

	switch requestType {
	case 0: // resources/list
		message = &models.MCPMessage{
			JSONRPC: "2.0",
			ID:      fmt.Sprintf("load-list-%d", time.Now().UnixNano()),
			Method:  "resources/list",
		}
	case 1: // resources/read
		message = &models.MCPMessage{
			JSONRPC: "2.0",
			ID:      fmt.Sprintf("load-read-%d", time.Now().UnixNano()),
			Method:  "resources/read",
			Params: models.MCPResourcesReadParams{
				URI: fmt.Sprintf("architecture://guidelines/load-test-%d", time.Now().UnixNano()%100),
			},
		}
	case 2: // server/performance
		message = &models.MCPMessage{
			JSONRPC: "2.0",
			ID:      fmt.Sprintf("load-perf-%d", time.Now().UnixNano()),
			Method:  "server/performance",
		}
	}

	response := mcpServer.HandleMessage(message)
	return response != nil && response.Error == nil
}

// calculateLoadTestResults calculates performance metrics from collected data
func calculateLoadTestResults(totalReqs, successfulReqs, failedReqs int64, responseTimes []time.Duration, duration time.Duration) LoadTestResults {
	if len(responseTimes) == 0 {
		return LoadTestResults{}
	}

	// Sort response times for percentile calculation
	for i := 0; i < len(responseTimes)-1; i++ {
		for j := i + 1; j < len(responseTimes); j++ {
			if responseTimes[i] > responseTimes[j] {
				responseTimes[i], responseTimes[j] = responseTimes[j], responseTimes[i]
			}
		}
	}

	// Calculate statistics
	var totalTime time.Duration
	for _, rt := range responseTimes {
		totalTime += rt
	}

	avgResponseTime := totalTime / time.Duration(len(responseTimes))
	minResponseTime := responseTimes[0]
	maxResponseTime := responseTimes[len(responseTimes)-1]
	p95ResponseTime := responseTimes[len(responseTimes)*95/100]
	p99ResponseTime := responseTimes[len(responseTimes)*99/100]

	requestsPerSecond := float64(totalReqs) / duration.Seconds()
	errorRate := float64(failedReqs) / float64(totalReqs) * 100.0

	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)

	return LoadTestResults{
		TotalRequests:     totalReqs,
		SuccessfulReqs:    successfulReqs,
		FailedReqs:        failedReqs,
		AvgResponseTime:   avgResponseTime,
		MinResponseTime:   minResponseTime,
		MaxResponseTime:   maxResponseTime,
		P95ResponseTime:   p95ResponseTime,
		P99ResponseTime:   p99ResponseTime,
		RequestsPerSecond: requestsPerSecond,
		ErrorRate:         errorRate,
		MemoryUsage:       memStats,
	}
}

// validateLoadTestResults validates that load test results meet performance requirements
func validateLoadTestResults(t *testing.T, config LoadTestConfig, results LoadTestResults) {
	// Validate error rate (should be < 1%)
	if results.ErrorRate > 1.0 {
		t.Errorf("Error rate too high: %.2f%% (expected < 1%%)", results.ErrorRate)
	}

	// Validate P95 response time (should be < 200ms for resources/list, < 500ms for resources/read)
	if results.P95ResponseTime > 200*time.Millisecond {
		t.Logf("P95 response time: %v (consider optimization if > 200ms)", results.P95ResponseTime)
	}

	// Validate throughput (should achieve at least 80% of target)
	expectedRPS := float64(config.RequestsPerSecond * config.ConcurrentClients)
	if results.RequestsPerSecond < expectedRPS*0.8 {
		t.Errorf("Throughput too low: %.2f RPS (expected at least %.2f RPS)",
			results.RequestsPerSecond, expectedRPS*0.8)
	}

	// Validate memory usage (should not exceed reasonable limits)
	memoryUsageMB := float64(results.MemoryUsage.Alloc) / 1024 / 1024
	expectedMemoryMB := float64(config.DocumentCount*config.DocumentSize) / 1024 / 1024 * 2 // 2x overhead
	if memoryUsageMB > expectedMemoryMB {
		t.Logf("Memory usage: %.2f MB (expected ~%.2f MB)", memoryUsageMB, expectedMemoryMB)
	}
}

// logLoadTestResults logs detailed load test results
func logLoadTestResults(t *testing.T, testName string, results LoadTestResults) {
	t.Logf("=== Load Test Results: %s ===", testName)
	t.Logf("Total Requests: %d", results.TotalRequests)
	t.Logf("Successful: %d (%.2f%%)", results.SuccessfulReqs,
		float64(results.SuccessfulReqs)/float64(results.TotalRequests)*100)
	t.Logf("Failed: %d (%.2f%%)", results.FailedReqs, results.ErrorRate)
	t.Logf("Requests/Second: %.2f", results.RequestsPerSecond)
	t.Logf("Response Times:")
	t.Logf("  Average: %v", results.AvgResponseTime)
	t.Logf("  Min: %v", results.MinResponseTime)
	t.Logf("  Max: %v", results.MaxResponseTime)
	t.Logf("  P95: %v", results.P95ResponseTime)
	t.Logf("  P99: %v", results.P99ResponseTime)
	t.Logf("Memory Usage:")
	t.Logf("  Allocated: %.2f MB", float64(results.MemoryUsage.Alloc)/1024/1024)
	t.Logf("  System: %.2f MB", float64(results.MemoryUsage.Sys)/1024/1024)
	t.Logf("  GC Cycles: %d", results.MemoryUsage.NumGC)
	t.Logf("=====================================")
}

// BenchmarkEndToEndPerformance benchmarks complete end-to-end performance
func BenchmarkEndToEndPerformance(b *testing.B) {
	// Create server with realistic document set
	mcpServer, cleanup := setupLoadTestServer(b, 1000, 5120)
	defer cleanup()

	// Test different request types
	testCases := []struct {
		name    string
		message *models.MCPMessage
	}{
		{
			name: "ResourcesList",
			message: &models.MCPMessage{
				JSONRPC: "2.0",
				ID:      "bench-list",
				Method:  "resources/list",
			},
		},
		{
			name: "ResourcesRead",
			message: &models.MCPMessage{
				JSONRPC: "2.0",
				ID:      "bench-read",
				Method:  "resources/read",
				Params: models.MCPResourcesReadParams{
					URI: "architecture://guidelines/load-test-0",
				},
			},
		},
		{
			name: "PerformanceMetrics",
			message: &models.MCPMessage{
				JSONRPC: "2.0",
				ID:      "bench-metrics",
				Method:  "server/performance",
			},
		},
	}

	for _, tc := range testCases {
		b.Run(tc.name, func(b *testing.B) {
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				response := mcpServer.HandleMessage(tc.message)
				if response == nil || response.Error != nil {
					b.Fatalf("Request failed: %v", response)
				}
			}
		})
	}
}

// Helper function for testing.B compatibility
func setupLoadTestServer(tb testing.TB, docCount, docSize int) (*server.MCPServer, func()) {
	// Create temporary directory for test documents
	tempDir, err := os.MkdirTemp("", "load_test")
	if err != nil {
		tb.Fatalf("Failed to create temp dir: %v", err)
	}

	// Create test documentation structure
	createLoadTestDocuments(tb, tempDir, docCount, docSize)

	// Create and initialize server
	mcpServer := server.NewMCPServer()

	// Change to temp directory for initialization
	originalDir, _ := os.Getwd()
	os.Chdir(tempDir)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Initialize documentation system without starting full server
	// (we'll call HandleMessage directly for testing)

	cleanup := func() {
		os.Chdir(originalDir)
		mcpServer.Shutdown(context.Background())
		os.RemoveAll(tempDir)
	}

	return mcpServer, cleanup
}

// Helper function for testing.TB compatibility
func createLoadTestDocuments(tb testing.TB, baseDir string, count, size int) {
	categories := []struct {
		name string
		dir  string
	}{
		{"guideline", "docs/guidelines"},
		{"pattern", "docs/patterns"},
		{"adr", "docs/adr"},
	}

	// Create directories
	for _, cat := range categories {
		dir := filepath.Join(baseDir, cat.dir)
		if err := os.MkdirAll(dir, 0755); err != nil {
			tb.Fatalf("Failed to create directory %s: %v", dir, err)
		}
	}

	// Create content template
	contentTemplate := strings.Repeat("This is test content for load testing. ", size/50)

	// Create documents
	for i := 0; i < count; i++ {
		cat := categories[i%len(categories)]

		content := fmt.Sprintf("# Load Test %s %d\n\n%s\n\n## Details\n\nDocument %d for load testing the MCP architecture service.\n\n%s",
			cat.name, i, contentTemplate, i, contentTemplate)

		filename := fmt.Sprintf("load-test-%d.md", i)
		filePath := filepath.Join(baseDir, cat.dir, filename)

		if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
			tb.Fatalf("Failed to create file %s: %v", filePath, err)
		}
	}
}
