package cache

import (
	"fmt"
	"runtime"
	"sync"
	"testing"
	"time"

	"mcp-architecture-service/internal/models"
)

// BenchmarkCacheSet benchmarks cache set operations with various document sizes
func BenchmarkCacheSet(b *testing.B) {
	testCases := []struct {
		name        string
		docSize     int
		concurrency int
	}{
		{"Small_1KB", 1024, 1},
		{"Medium_10KB", 10240, 1},
		{"Large_100KB", 102400, 1},
		{"Small_1KB_Concurrent", 1024, 10},
		{"Medium_10KB_Concurrent", 10240, 10},
	}

	for _, tc := range testCases {
		b.Run(tc.name, func(b *testing.B) {
			cache := NewDocumentCache()
			defer cache.Close()

			// Create test content of specified size
			content := make([]byte, tc.docSize)
			for i := range content {
				content[i] = byte('A' + (i % 26))
			}

			if tc.concurrency == 1 {
				// Sequential benchmark
				b.ResetTimer()
				for i := 0; i < b.N; i++ {
					doc := createTestDocument(i, string(content))
					cache.Set(doc.Metadata.Path, doc)
				}
			} else {
				// Concurrent benchmark
				b.ResetTimer()
				b.RunParallel(func(pb *testing.PB) {
					i := 0
					for pb.Next() {
						doc := createTestDocument(i, string(content))
						cache.Set(doc.Metadata.Path, doc)
						i++
					}
				})
			}
		})
	}
}

// BenchmarkCacheGet benchmarks cache get operations with various cache sizes
func BenchmarkCacheGet(b *testing.B) {
	testCases := []struct {
		name      string
		cacheSize int
	}{
		{"Small_100", 100},
		{"Medium_1000", 1000},
		{"Large_10000", 10000},
		{"XLarge_50000", 50000},
	}

	for _, tc := range testCases {
		b.Run(tc.name, func(b *testing.B) {
			cache := NewDocumentCache()
			defer cache.Close()

			// Populate cache
			keys := make([]string, tc.cacheSize)
			for i := 0; i < tc.cacheSize; i++ {
				doc := createTestDocument(i, fmt.Sprintf("Content for document %d", i))
				cache.Set(doc.Metadata.Path, doc)
				keys[i] = doc.Metadata.Path
			}

			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				key := keys[i%len(keys)]
				_, _ = cache.Get(key)
			}
		})
	}
}

// BenchmarkCacheGetConcurrent benchmarks concurrent cache get operations
func BenchmarkCacheGetConcurrent(b *testing.B) {
	cache := NewDocumentCache()
	defer cache.Close()

	// Populate cache with test documents
	const cacheSize = 1000
	keys := make([]string, cacheSize)
	for i := 0; i < cacheSize; i++ {
		doc := createTestDocument(i, fmt.Sprintf("Content for document %d", i))
		cache.Set(doc.Metadata.Path, doc)
		keys[i] = doc.Metadata.Path
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			key := keys[i%len(keys)]
			_, _ = cache.Get(key)
			i++
		}
	})
}

// BenchmarkCacheGetByCategory benchmarks category-based retrieval
func BenchmarkCacheGetByCategory(b *testing.B) {
	testCases := []struct {
		name         string
		totalDocs    int
		docsPerCat   int
		categoryName string
	}{
		{"Small_100_docs", 100, 33, "guideline"},
		{"Medium_1000_docs", 1000, 333, "guideline"},
		{"Large_10000_docs", 10000, 3333, "guideline"},
	}

	for _, tc := range testCases {
		b.Run(tc.name, func(b *testing.B) {
			cache := NewDocumentCache()
			defer cache.Close()

			// Populate cache with documents across multiple categories
			categories := []string{"guideline", "pattern", "adr"}
			for i := 0; i < tc.totalDocs; i++ {
				category := categories[i%len(categories)]
				doc := createTestDocumentWithCategory(i, category, fmt.Sprintf("Content %d", i))
				cache.Set(doc.Metadata.Path, doc)
			}

			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				_ = cache.GetByCategory(tc.categoryName)
			}
		})
	}
}

// BenchmarkCacheInvalidate benchmarks cache invalidation operations
func BenchmarkCacheInvalidate(b *testing.B) {
	cache := NewDocumentCache()
	defer cache.Close()

	// Pre-populate cache
	const cacheSize = 10000
	keys := make([]string, cacheSize)
	for i := 0; i < cacheSize; i++ {
		doc := createTestDocument(i, fmt.Sprintf("Content %d", i))
		cache.Set(doc.Metadata.Path, doc)
		keys[i] = doc.Metadata.Path
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// Invalidate and re-add to maintain cache size
		key := keys[i%len(keys)]
		cache.Invalidate(key)

		// Re-add the document
		doc := createTestDocument(i, fmt.Sprintf("New content %d", i))
		cache.Set(key, doc)
	}
}

// BenchmarkCacheInvalidateByCategory benchmarks category-based invalidation
func BenchmarkCacheInvalidateByCategory(b *testing.B) {
	testCases := []struct {
		name      string
		totalDocs int
	}{
		{"Small_300_docs", 300},
		{"Medium_3000_docs", 3000},
		{"Large_30000_docs", 30000},
	}

	for _, tc := range testCases {
		b.Run(tc.name, func(b *testing.B) {
			cache := NewDocumentCache()
			defer cache.Close()

			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				b.StopTimer()
				// Repopulate cache for each iteration
				categories := []string{"guideline", "pattern", "adr"}
				for j := 0; j < tc.totalDocs; j++ {
					category := categories[j%len(categories)]
					doc := createTestDocumentWithCategory(j, category, fmt.Sprintf("Content %d", j))
					cache.Set(doc.Metadata.Path, doc)
				}
				b.StartTimer()

				// Benchmark the invalidation
				cache.InvalidateByCategory("guideline")
			}
		})
	}
}

// BenchmarkCacheMemoryUsage benchmarks memory usage and cleanup operations
func BenchmarkCacheMemoryUsage(b *testing.B) {
	testCases := []struct {
		name     string
		docSize  int
		docCount int
	}{
		{"Small_docs_1KB", 1024, 1000},
		{"Medium_docs_10KB", 10240, 1000},
		{"Large_docs_100KB", 102400, 100},
	}

	for _, tc := range testCases {
		b.Run(tc.name, func(b *testing.B) {
			var m1, m2 runtime.MemStats

			// Measure initial memory
			runtime.GC()
			runtime.ReadMemStats(&m1)

			cache := NewDocumentCache()
			defer cache.Close()

			// Create content of specified size
			content := make([]byte, tc.docSize)
			for i := range content {
				content[i] = byte('A' + (i % 26))
			}

			// Populate cache
			for i := 0; i < tc.docCount; i++ {
				doc := createTestDocument(i, string(content))
				cache.Set(doc.Metadata.Path, doc)
			}

			// Measure memory after population
			runtime.GC()
			runtime.ReadMemStats(&m2)

			memoryUsed := m2.Alloc - m1.Alloc
			b.ReportMetric(float64(memoryUsed), "bytes_used")
			b.ReportMetric(float64(memoryUsed)/float64(tc.docCount), "bytes_per_doc")

			// Benchmark cleanup operations
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				cache.Cleanup()
			}
		})
	}
}

// BenchmarkCacheStats benchmarks statistics collection
func BenchmarkCacheStats(b *testing.B) {
	cache := NewDocumentCache()
	defer cache.Close()

	// Populate cache and generate some stats
	for i := 0; i < 1000; i++ {
		doc := createTestDocument(i, fmt.Sprintf("Content %d", i))
		cache.Set(doc.Metadata.Path, doc)
	}

	// Generate some hits and misses
	for i := 0; i < 100; i++ {
		cache.Get(fmt.Sprintf("docs/guidelines/test-%d.md", i))
		cache.Get(fmt.Sprintf("nonexistent-%d", i))
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = cache.GetStats()
	}
}

// BenchmarkCacheGetPerformanceMetrics benchmarks performance metrics collection
func BenchmarkCacheGetPerformanceMetrics(b *testing.B) {
	cache := NewDocumentCache()
	defer cache.Close()

	// Populate cache
	for i := 0; i < 1000; i++ {
		doc := createTestDocument(i, fmt.Sprintf("Content %d", i))
		cache.Set(doc.Metadata.Path, doc)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = cache.GetPerformanceMetrics()
	}
}

// BenchmarkCacheConcurrentMixedOperations benchmarks mixed concurrent operations
func BenchmarkCacheConcurrentMixedOperations(b *testing.B) {
	cache := NewDocumentCache()
	defer cache.Close()

	// Pre-populate cache
	const initialSize = 1000
	for i := 0; i < initialSize; i++ {
		doc := createTestDocument(i, fmt.Sprintf("Initial content %d", i))
		cache.Set(doc.Metadata.Path, doc)
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			operation := i % 4
			switch operation {
			case 0: // Get operation (50% of operations)
				key := fmt.Sprintf("docs/guidelines/test-%d.md", i%initialSize)
				cache.Get(key)
			case 1: // Set operation (25% of operations)
				doc := createTestDocument(i+initialSize, fmt.Sprintf("New content %d", i))
				cache.Set(doc.Metadata.Path, doc)
			case 2: // GetByCategory operation (15% of operations)
				cache.GetByCategory("guideline")
			case 3: // Stats operation (10% of operations)
				cache.GetStats()
			}
			i++
		}
	})
}

// BenchmarkCacheScalability tests cache performance at different scales
func BenchmarkCacheScalability(b *testing.B) {
	scales := []int{100, 1000, 10000, 50000}

	for _, scale := range scales {
		b.Run(fmt.Sprintf("Scale_%d", scale), func(b *testing.B) {
			cache := NewDocumentCache()
			defer cache.Close()

			// Populate cache to the target scale
			for i := 0; i < scale; i++ {
				doc := createTestDocument(i, fmt.Sprintf("Content %d", i))
				cache.Set(doc.Metadata.Path, doc)
			}

			// Benchmark get operations at this scale
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				key := fmt.Sprintf("docs/guidelines/test-%d.md", i%scale)
				cache.Get(key)
			}
		})
	}
}

// LoadTestCacheHighConcurrency performs load testing with high concurrency
func LoadTestCacheHighConcurrency(b *testing.B) {
	concurrencyLevels := []int{10, 50, 100, 200}

	for _, concurrency := range concurrencyLevels {
		b.Run(fmt.Sprintf("Concurrency_%d", concurrency), func(b *testing.B) {
			cache := NewDocumentCache()
			defer cache.Close()

			// Pre-populate cache
			const cacheSize = 10000
			for i := 0; i < cacheSize; i++ {
				doc := createTestDocument(i, fmt.Sprintf("Content %d", i))
				cache.Set(doc.Metadata.Path, doc)
			}

			var wg sync.WaitGroup
			operationsPerWorker := b.N / concurrency
			if operationsPerWorker == 0 {
				operationsPerWorker = 1
			}

			b.ResetTimer()
			startTime := time.Now()

			for i := 0; i < concurrency; i++ {
				wg.Add(1)
				go func(workerID int) {
					defer wg.Done()
					for j := 0; j < operationsPerWorker; j++ {
						// Mix of operations: 70% reads, 20% writes, 10% category queries
						op := j % 10
						if op < 7 {
							// Read operation
							key := fmt.Sprintf("docs/guidelines/test-%d.md", j%cacheSize)
							cache.Get(key)
						} else if op < 9 {
							// Write operation
							doc := createTestDocument(j+cacheSize, fmt.Sprintf("New content %d-%d", workerID, j))
							cache.Set(doc.Metadata.Path, doc)
						} else {
							// Category query
							cache.GetByCategory("guideline")
						}
					}
				}(i)
			}

			wg.Wait()
			duration := time.Since(startTime)

			totalOps := concurrency * operationsPerWorker
			opsPerSecond := float64(totalOps) / duration.Seconds()
			b.ReportMetric(opsPerSecond, "ops/sec")
		})
	}
}

// BenchmarkCacheMemoryEfficiency tests memory efficiency under various conditions
func BenchmarkCacheMemoryEfficiency(b *testing.B) {
	testCases := []struct {
		name           string
		docCount       int
		docSize        int
		operationCount int
	}{
		{"Efficient_small", 1000, 1024, 10000},
		{"Efficient_medium", 5000, 5120, 50000},
		{"Efficient_large", 1000, 51200, 10000},
	}

	for _, tc := range testCases {
		b.Run(tc.name, func(b *testing.B) {
			var m1, m2, m3 runtime.MemStats

			// Measure baseline memory
			runtime.GC()
			runtime.ReadMemStats(&m1)

			cache := NewDocumentCache()
			defer cache.Close()

			// Create content
			content := make([]byte, tc.docSize)
			for i := range content {
				content[i] = byte('A' + (i % 26))
			}

			// Populate cache
			for i := 0; i < tc.docCount; i++ {
				doc := createTestDocument(i, string(content))
				cache.Set(doc.Metadata.Path, doc)
			}

			// Measure memory after population
			runtime.GC()
			runtime.ReadMemStats(&m2)

			b.ResetTimer()
			// Perform operations
			for i := 0; i < b.N; i++ {
				for j := 0; j < tc.operationCount; j++ {
					// Mix of operations
					switch j % 5 {
					case 0, 1, 2: // 60% reads
						key := fmt.Sprintf("docs/guidelines/test-%d.md", j%tc.docCount)
						cache.Get(key)
					case 3: // 20% category queries
						cache.GetByCategory("guideline")
					case 4: // 20% stats
						cache.GetStats()
					}
				}
			}

			// Measure final memory
			runtime.GC()
			runtime.ReadMemStats(&m3)

			initialMemory := m2.Alloc - m1.Alloc
			finalMemory := m3.Alloc - m1.Alloc
			memoryGrowth := finalMemory - initialMemory

			b.ReportMetric(float64(initialMemory), "initial_bytes")
			b.ReportMetric(float64(finalMemory), "final_bytes")
			b.ReportMetric(float64(memoryGrowth), "memory_growth_bytes")
			b.ReportMetric(float64(initialMemory)/float64(tc.docCount), "bytes_per_doc")
		})
	}
}

// Helper function to create a test document
func createTestDocument(id int, content string) *models.Document {
	return &models.Document{
		Metadata: models.DocumentMetadata{
			Title:        fmt.Sprintf("Test Document %d", id),
			Category:     "guideline",
			Path:         fmt.Sprintf("docs/guidelines/test-%d.md", id),
			LastModified: time.Now(),
			Size:         int64(len(content)),
			Checksum:     fmt.Sprintf("checksum-%d", id),
		},
		Content: models.DocumentContent{
			RawContent: content,
		},
	}
}

// Helper function to create a test document with specific category
func createTestDocumentWithCategory(id int, category, content string) *models.Document {
	return &models.Document{
		Metadata: models.DocumentMetadata{
			Title:        fmt.Sprintf("Test %s %d", category, id),
			Category:     category,
			Path:         fmt.Sprintf("docs/%ss/test-%d.md", category, id),
			LastModified: time.Now(),
			Size:         int64(len(content)),
			Checksum:     fmt.Sprintf("checksum-%s-%d", category, id),
		},
		Content: models.DocumentContent{
			RawContent: content,
		},
	}
}
