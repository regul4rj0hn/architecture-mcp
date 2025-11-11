package server

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"mcp-architecture-service/internal/models"
	"mcp-architecture-service/pkg/config"
)

// BenchmarkMCPInitialize benchmarks the MCP initialization flow
func BenchmarkMCPInitialize(b *testing.B) {
	server := newMCPServerWithOptions(false) // Disable file monitor for benchmarks

	initMessage := &models.MCPMessage{
		JSONRPC: "2.0",
		ID:      "bench-init",
		Method:  "initialize",
		Params: models.MCPInitializeParams{
			ProtocolVersion: "2024-11-05",
			Capabilities:    map[string]interface{}{},
			ClientInfo: models.MCPClientInfo{
				Name:    "benchmark-client",
				Version: "1.0.0",
			},
		},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		response := server.handleInitialize(initMessage)
		if response == nil || response.Error != nil {
			b.Fatalf("Initialize failed: %v", response)
		}
	}
}

// BenchmarkResourcesList benchmarks the resources/list method with various cache sizes
func BenchmarkResourcesList(b *testing.B) {
	testCases := []struct {
		name     string
		docCount int
	}{
		{"Small_10_docs", 10},
		{"Medium_100_docs", 100},
		{"Large_1000_docs", 1000},
	}

	for _, tc := range testCases {
		b.Run(tc.name, func(b *testing.B) {
			server := newMCPServerWithOptions(false) // Disable file monitor for benchmarks
			populateServerWithTestDocuments(server, tc.docCount)

			listMessage := &models.MCPMessage{
				JSONRPC: "2.0",
				ID:      "bench-list",
				Method:  "resources/list",
			}

			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				response := server.handleResourcesList(listMessage)
				if response == nil || response.Error != nil {
					b.Fatalf("ResourcesList failed: %v", response)
				}
			}
		})
	}
}

// BenchmarkResourcesRead benchmarks the resources/read method
func BenchmarkResourcesRead(b *testing.B) {
	server := newMCPServerWithOptions(false) // Disable file monitor for benchmarks
	populateServerWithTestDocuments(server, 100)

	readMessage := &models.MCPMessage{
		JSONRPC: "2.0",
		ID:      "bench-read",
		Method:  "resources/read",
		Params: models.MCPResourcesReadParams{
			URI: "architecture://guidelines/test-0",
		},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		response := server.handleResourcesRead(readMessage)
		if response == nil || response.Error != nil {
			b.Fatalf("ResourcesRead failed: %v", response)
		}
	}
}

// BenchmarkConcurrentResourcesRead benchmarks concurrent resource read operations
func BenchmarkConcurrentResourcesRead(b *testing.B) {
	server := newMCPServerWithOptions(false) // Disable file monitor for benchmarks
	populateServerWithTestDocuments(server, 100)

	// Create multiple URIs to read
	uris := make([]string, 100)
	for i := 0; i < 100; i++ {
		uris[i] = fmt.Sprintf("architecture://guidelines/test-%d", i)
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		uriIndex := 0
		for pb.Next() {
			readMessage := &models.MCPMessage{
				JSONRPC: "2.0",
				ID:      fmt.Sprintf("bench-read-%d", uriIndex),
				Method:  "resources/read",
				Params: models.MCPResourcesReadParams{
					URI: uris[uriIndex%len(uris)],
				},
			}

			response := server.handleResourcesRead(readMessage)
			if response == nil || response.Error != nil {
				b.Fatalf("ResourcesRead failed: %v", response)
			}
			uriIndex++
		}
	})
}

// BenchmarkJSONRPCProcessing benchmarks the complete JSON-RPC message processing pipeline
func BenchmarkJSONRPCProcessing(b *testing.B) {
	server := newMCPServerWithOptions(false) // Disable file monitor for benchmarks
	populateServerWithTestDocuments(server, 100)

	// Create a list request message
	listJSON := `{"jsonrpc":"2.0","id":"bench","method":"resources/list"}`

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		reader := strings.NewReader(listJSON)
		writer := &bytes.Buffer{}

		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		err := server.processMessages(ctx, reader, writer)
		cancel()

		if err != nil && err != context.DeadlineExceeded {
			b.Fatalf("processMessages failed: %v", err)
		}

		if writer.Len() == 0 {
			b.Fatal("No response written")
		}
	}
}

// BenchmarkMemoryUsageWithLargeDataset benchmarks memory usage with large documentation sets
func BenchmarkMemoryUsageWithLargeDataset(b *testing.B) {
	testCases := []struct {
		name     string
		docCount int
		docSize  int // Size of each document in bytes
	}{
		{"Small_docs_1KB", 100, 1024},
		{"Medium_docs_10KB", 50, 10240},
		{"Large_docs_100KB", 20, 102400},
	}

	for _, tc := range testCases {
		b.Run(tc.name, func(b *testing.B) {
			var m1, m2 runtime.MemStats
			runtime.GC()
			runtime.ReadMemStats(&m1)

			server := newMCPServerWithOptions(false) // Disable file monitor for benchmarks
			populateServerWithLargeTestDocuments(server, tc.docCount, tc.docSize)

			runtime.GC()
			runtime.ReadMemStats(&m2)

			memoryUsed := m2.Alloc - m1.Alloc
			b.ReportMetric(float64(memoryUsed), "bytes/op")
			b.ReportMetric(float64(memoryUsed)/float64(tc.docCount), "bytes/doc")

			// Benchmark resource list performance with large dataset
			listMessage := &models.MCPMessage{
				JSONRPC: "2.0",
				ID:      "bench-list",
				Method:  "resources/list",
			}

			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				response := server.handleResourcesList(listMessage)
				if response == nil || response.Error != nil {
					b.Fatalf("ResourcesList failed: %v", response)
				}
			}
		})
	}
}

// BenchmarkCacheOperations benchmarks cache performance with various operations
func BenchmarkCacheOperations(b *testing.B) {
	server := newMCPServerWithOptions(false) // Disable file monitor for benchmarks
	populateServerWithTestDocuments(server, 100)

	b.Run("CacheGet", func(b *testing.B) {
		keys := make([]string, 100)
		for i := 0; i < 100; i++ {
			keys[i] = fmt.Sprintf("%s/test-%d.md", config.GuidelinesPath, i)
		}

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			key := keys[i%len(keys)]
			_, _ = server.cache.Get(key)
		}
	})

	b.Run("CacheGetByCategory", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_ = server.cache.GetByCategory("guideline")
		}
	})

	b.Run("CacheStats", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_ = server.cache.GetStats()
		}
	})
}

// BenchmarkStartupTime benchmarks server startup time with various documentation sizes
func BenchmarkStartupTime(b *testing.B) {
	// Create temporary directory with test documents
	tempDir, err := os.MkdirTemp("", "benchmark_startup")
	if err != nil {
		b.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	testCases := []struct {
		name     string
		docCount int
	}{
		{"Startup_10_docs", 10},
		{"Startup_50_docs", 50},
		{"Startup_100_docs", 100},
	}

	for _, tc := range testCases {
		b.Run(tc.name, func(b *testing.B) {
			// Create test documentation files
			createTestDocumentationFiles(tempDir, tc.docCount)

			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				server := newMCPServerWithOptions(false) // Disable file monitor for benchmarks

				// Change to temp directory for scanning
				originalDir, _ := os.Getwd()
				os.Chdir(tempDir)

				ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
				err := server.initializeDocumentationSystem(ctx)
				cancel()

				os.Chdir(originalDir)

				if err != nil {
					b.Fatalf("Failed to initialize documentation system: %v", err)
				}

				// Verify documents were loaded
				if server.cache.Size() == 0 {
					b.Fatal("No documents loaded during startup")
				}
			}
		})
	}
}

// LoadTestConcurrentRequests performs load testing with concurrent MCP requests
func LoadTestConcurrentRequests(b *testing.B) {
	server := newMCPServerWithOptions(false) // Disable file monitor for benchmarks
	populateServerWithTestDocuments(server, 1000)

	concurrencyLevels := []int{1, 5, 10, 25, 50, 100}

	for _, concurrency := range concurrencyLevels {
		b.Run(fmt.Sprintf("Concurrency_%d", concurrency), func(b *testing.B) {
			var wg sync.WaitGroup
			requestsPerWorker := b.N / concurrency
			if requestsPerWorker == 0 {
				requestsPerWorker = 1
			}

			b.ResetTimer()
			startTime := time.Now()

			for i := 0; i < concurrency; i++ {
				wg.Add(1)
				go func(workerID int) {
					defer wg.Done()

					for j := 0; j < requestsPerWorker; j++ {
						// Alternate between list and read operations
						if j%2 == 0 {
							listMessage := &models.MCPMessage{
								JSONRPC: "2.0",
								ID:      fmt.Sprintf("load-list-%d-%d", workerID, j),
								Method:  "resources/list",
							}
							response := server.handleResourcesList(listMessage)
							if response == nil || response.Error != nil {
								b.Errorf("ResourcesList failed: %v", response)
								return
							}
						} else {
							readMessage := &models.MCPMessage{
								JSONRPC: "2.0",
								ID:      fmt.Sprintf("load-read-%d-%d", workerID, j),
								Method:  "resources/read",
								Params: models.MCPResourcesReadParams{
									URI: fmt.Sprintf("architecture://guidelines/test-%d", j%100),
								},
							}
							response := server.handleResourcesRead(readMessage)
							if response == nil || response.Error != nil {
								b.Errorf("ResourcesRead failed: %v", response)
								return
							}
						}
					}
				}(i)
			}

			wg.Wait()
			duration := time.Since(startTime)

			totalRequests := concurrency * requestsPerWorker
			requestsPerSecond := float64(totalRequests) / duration.Seconds()
			b.ReportMetric(requestsPerSecond, "requests/sec")
			b.ReportMetric(duration.Seconds()/float64(totalRequests)*1000, "ms/request")
		})
	}
}

// BenchmarkPerformanceMetrics benchmarks the performance metrics endpoint
func BenchmarkPerformanceMetrics(b *testing.B) {
	server := newMCPServerWithOptions(false) // Disable file monitor for benchmarks
	populateServerWithTestDocuments(server, 100)

	metricsMessage := &models.MCPMessage{
		JSONRPC: "2.0",
		ID:      "bench-metrics",
		Method:  "server/performance",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		response := server.handlePerformanceMetrics(metricsMessage)
		if response == nil || response.Error != nil {
			b.Fatalf("PerformanceMetrics failed: %v", response)
		}
	}
}

// Helper function to populate server with test documents
func populateServerWithTestDocuments(server *MCPServer, count int) {
	categoryPaths := map[string]string{
		"guideline": config.GuidelinesPath,
		"pattern":   config.PatternsPath,
		"adr":       config.ADRPath,
	}
	categories := []string{"guideline", "pattern", "adr"}

	for i := 0; i < count; i++ {
		category := categories[i%len(categories)]
		basePath := categoryPaths[category]

		doc := &models.Document{
			Metadata: models.DocumentMetadata{
				Title:        fmt.Sprintf("Test %s %d", category, i),
				Category:     category,
				Path:         fmt.Sprintf("%s/test-%d.md", basePath, i),
				LastModified: time.Now(),
				Size:         1024,
				Checksum:     fmt.Sprintf("checksum-%d", i),
			},
			Content: models.DocumentContent{
				RawContent: fmt.Sprintf("# Test %s %d\n\nThis is test content for %s document %d.\n\n## Section 1\n\nSome content here.\n\n## Section 2\n\nMore content here.", category, i, category, i),
			},
		}

		server.cache.Set(doc.Metadata.Path, doc)
	}
}

// Helper function to populate server with large test documents
func populateServerWithLargeTestDocuments(server *MCPServer, count int, docSize int) {
	categoryPaths := map[string]string{
		"guideline": config.GuidelinesPath,
		"pattern":   config.PatternsPath,
		"adr":       config.ADRPath,
	}
	categories := []string{"guideline", "pattern", "adr"}

	// Create content of specified size
	contentTemplate := strings.Repeat("This is test content for performance testing. ", docSize/50)

	for i := 0; i < count; i++ {
		category := categories[i%len(categories)]
		basePath := categoryPaths[category]

		doc := &models.Document{
			Metadata: models.DocumentMetadata{
				Title:        fmt.Sprintf("Large Test %s %d", category, i),
				Category:     category,
				Path:         fmt.Sprintf("%s/large-test-%d.md", basePath, i),
				LastModified: time.Now(),
				Size:         int64(len(contentTemplate)),
				Checksum:     fmt.Sprintf("large-checksum-%d", i),
			},
			Content: models.DocumentContent{
				RawContent: fmt.Sprintf("# Large Test %s %d\n\n%s", category, i, contentTemplate),
			},
		}

		server.cache.Set(doc.Metadata.Path, doc)
	}
}

// Helper function to create test documentation files on disk
func createTestDocumentationFiles(baseDir string, count int) error {
	categories := []struct {
		name string
		dir  string
	}{
		{"guideline", config.GuidelinesPath},
		{"pattern", config.PatternsPath},
		{"adr", config.ADRPath},
	}

	for _, cat := range categories {
		dir := filepath.Join(baseDir, cat.dir)
		if err := os.MkdirAll(dir, 0755); err != nil {
			return err
		}
	}

	for i := 0; i < count; i++ {
		cat := categories[i%len(categories)]

		content := fmt.Sprintf("# Test %s %d\n\nThis is test content for %s document %d.\n\n## Overview\n\nDetailed information about this %s.\n\n## Implementation\n\nImplementation details here.\n\n## Examples\n\nCode examples and usage patterns.",
			cat.name, i, cat.name, i, cat.name)

		filename := fmt.Sprintf("test-%d.md", i)
		filePath := filepath.Join(baseDir, cat.dir, filename)

		if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
			return err
		}
	}

	return nil
}

// BenchmarkResponseTimePercentiles measures response time percentiles
func BenchmarkResponseTimePercentiles(b *testing.B) {
	server := newMCPServerWithOptions(false) // Disable file monitor for benchmarks
	populateServerWithTestDocuments(server, 100)

	// Collect response times
	responseTimes := make([]time.Duration, b.N)

	listMessage := &models.MCPMessage{
		JSONRPC: "2.0",
		ID:      "bench-percentiles",
		Method:  "resources/list",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		start := time.Now()
		response := server.handleResourcesList(listMessage)
		responseTimes[i] = time.Since(start)

		if response == nil || response.Error != nil {
			b.Fatalf("ResourcesList failed: %v", response)
		}
	}

	// Calculate percentiles (simple implementation)
	if b.N > 0 {
		// Sort response times for percentile calculation
		for i := 0; i < len(responseTimes)-1; i++ {
			for j := i + 1; j < len(responseTimes); j++ {
				if responseTimes[i] > responseTimes[j] {
					responseTimes[i], responseTimes[j] = responseTimes[j], responseTimes[i]
				}
			}
		}

		p50 := responseTimes[len(responseTimes)*50/100]
		p95 := responseTimes[len(responseTimes)*95/100]
		p99 := responseTimes[len(responseTimes)*99/100]

		b.ReportMetric(float64(p50.Nanoseconds())/1e6, "p50_ms")
		b.ReportMetric(float64(p95.Nanoseconds())/1e6, "p95_ms")
		b.ReportMetric(float64(p99.Nanoseconds())/1e6, "p99_ms")
	}
}
