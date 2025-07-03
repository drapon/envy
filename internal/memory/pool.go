package memory

import (
	"runtime"
	"sync"
	"sync/atomic"
	"time"
)

// PoolStats holds statistics about pool usage
type PoolStats struct {
	Gets      int64
	Puts      int64
	Hits      int64
	Misses    int64
	Size      int64
	Capacity  int64
	MemUsage  int64
}

// StringPool manages a pool of strings for memory efficiency
type StringPool struct {
	pool      sync.Pool
	stats     *PoolStats
	maxSize   int64
	allocated int64
}

// NewStringPool creates a new string pool
func NewStringPool(maxSize int64) *StringPool {
	return &StringPool{
		pool: sync.Pool{
			New: func() interface{} {
				return make([]string, 0, 16)
			},
		},
		stats:   &PoolStats{Capacity: maxSize},
		maxSize: maxSize,
	}
}

// Get returns a string slice from the pool
func (p *StringPool) Get() []string {
	atomic.AddInt64(&p.stats.Gets, 1)
	slice := p.pool.Get().([]string)
	if len(slice) == 0 {
		atomic.AddInt64(&p.stats.Misses, 1)
		return make([]string, 0, 16)
	}
	atomic.AddInt64(&p.stats.Hits, 1)
	return slice[:0] // Reset length but keep capacity
}

// Put returns a string slice to the pool
func (p *StringPool) Put(s []string) {
	atomic.AddInt64(&p.stats.Puts, 1)
	if cap(s) > 0 && int64(cap(s)) <= p.maxSize {
		atomic.AddInt64(&p.stats.Size, 1)
		p.pool.Put(s)
	}
}

// Stats returns current pool statistics
func (p *StringPool) Stats() PoolStats {
	return PoolStats{
		Gets:     atomic.LoadInt64(&p.stats.Gets),
		Puts:     atomic.LoadInt64(&p.stats.Puts),
		Hits:     atomic.LoadInt64(&p.stats.Hits),
		Misses:   atomic.LoadInt64(&p.stats.Misses),
		Size:     atomic.LoadInt64(&p.stats.Size),
		Capacity: p.stats.Capacity,
		MemUsage: atomic.LoadInt64(&p.stats.MemUsage),
	}
}

// BytePool manages a pool of byte slices for memory efficiency
type BytePool struct {
	pools     []*sync.Pool
	sizes     []int
	stats     *PoolStats
	maxSize   int64
	allocated int64
}

// NewBytePool creates a new byte pool with predefined sizes
func NewBytePool(maxSize int64) *BytePool {
	sizes := []int{64, 256, 1024, 4096, 16384, 65536}
	pools := make([]*sync.Pool, len(sizes))
	
	for i, size := range sizes {
		size := size // Capture for closure
		pools[i] = &sync.Pool{
			New: func() interface{} {
				return make([]byte, size)
			},
		}
	}
	
	return &BytePool{
		pools:   pools,
		sizes:   sizes,
		stats:   &PoolStats{Capacity: maxSize},
		maxSize: maxSize,
	}
}

// Get returns a byte slice from the pool
func (p *BytePool) Get(size int) []byte {
	atomic.AddInt64(&p.stats.Gets, 1)
	
	// Find the smallest pool that can accommodate the size
	for i, poolSize := range p.sizes {
		if size <= poolSize {
			atomic.AddInt64(&p.stats.Hits, 1)
			return p.pools[i].Get().([]byte)[:size]
		}
	}
	
	// Size too large for pools, allocate directly
	atomic.AddInt64(&p.stats.Misses, 1)
	return make([]byte, size)
}

// Put returns a byte slice to the pool
func (p *BytePool) Put(b []byte) {
	atomic.AddInt64(&p.stats.Puts, 1)
	
	size := cap(b)
	if size > int(p.maxSize) {
		return
	}
	
	// Find the appropriate pool
	for i, poolSize := range p.sizes {
		if size <= poolSize {
			atomic.AddInt64(&p.stats.Size, 1)
			p.pools[i].Put(b[:poolSize])
			return
		}
	}
}

// Stats returns current pool statistics
func (p *BytePool) Stats() PoolStats {
	return PoolStats{
		Gets:     atomic.LoadInt64(&p.stats.Gets),
		Puts:     atomic.LoadInt64(&p.stats.Puts),
		Hits:     atomic.LoadInt64(&p.stats.Hits),
		Misses:   atomic.LoadInt64(&p.stats.Misses),
		Size:     atomic.LoadInt64(&p.stats.Size),
		Capacity: p.stats.Capacity,
		MemUsage: atomic.LoadInt64(&p.stats.MemUsage),
	}
}

// MapPool manages a pool of string maps for memory efficiency
type MapPool struct {
	pool      sync.Pool
	stats     *PoolStats
	maxSize   int64
	allocated int64
}

// NewMapPool creates a new map pool
func NewMapPool(maxSize int64) *MapPool {
	return &MapPool{
		pool: sync.Pool{
			New: func() interface{} {
				return make(map[string]string, 16)
			},
		},
		stats:   &PoolStats{Capacity: maxSize},
		maxSize: maxSize,
	}
}

// Get returns a map from the pool
func (p *MapPool) Get() map[string]string {
	atomic.AddInt64(&p.stats.Gets, 1)
	m := p.pool.Get().(map[string]string)
	
	// Clear the map
	for k := range m {
		delete(m, k)
	}
	
	atomic.AddInt64(&p.stats.Hits, 1)
	return m
}

// Put returns a map to the pool
func (p *MapPool) Put(m map[string]string) {
	atomic.AddInt64(&p.stats.Puts, 1)
	
	// Don't pool maps that are too large
	if int64(len(m)) > p.maxSize {
		return
	}
	
	// Clear the map before returning to pool
	for k := range m {
		delete(m, k)
	}
	
	atomic.AddInt64(&p.stats.Size, 1)
	p.pool.Put(m)
}

// Stats returns current pool statistics
func (p *MapPool) Stats() PoolStats {
	return PoolStats{
		Gets:     atomic.LoadInt64(&p.stats.Gets),
		Puts:     atomic.LoadInt64(&p.stats.Puts),
		Hits:     atomic.LoadInt64(&p.stats.Hits),
		Misses:   atomic.LoadInt64(&p.stats.Misses),
		Size:     atomic.LoadInt64(&p.stats.Size),
		Capacity: p.stats.Capacity,
		MemUsage: atomic.LoadInt64(&p.stats.MemUsage),
	}
}

// PoolManager manages all pools and provides memory monitoring
type PoolManager struct {
	stringPool *StringPool
	bytePool   *BytePool
	mapPool    *MapPool
	enabled    bool
	monitoring bool
	gcInterval time.Duration
	done       chan bool
	stats      *MemoryStats
	mu         sync.RWMutex
}

// MemoryStats holds overall memory statistics
type MemoryStats struct {
	Alloc        uint64
	TotalAlloc   uint64
	Sys          uint64
	Lookups      uint64
	Mallocs      uint64
	Frees        uint64
	HeapAlloc    uint64
	HeapSys      uint64
	HeapIdle     uint64
	HeapInuse    uint64
	HeapReleased uint64
	StackInuse   uint64
	StackSys     uint64
	GCCPUFraction float64
	NumGC        uint32
	LastGC       time.Time
}

// NewPoolManager creates a new pool manager
func NewPoolManager(config PoolConfig) *PoolManager {
	manager := &PoolManager{
		stringPool: NewStringPool(config.StringPoolSize),
		bytePool:   NewBytePool(config.BytePoolSize),
		mapPool:    NewMapPool(config.MapPoolSize),
		enabled:    config.Enabled,
		monitoring: config.EnableMonitoring,
		gcInterval: config.GCInterval,
		done:       make(chan bool),
		stats:      &MemoryStats{},
	}
	
	if manager.monitoring {
		go manager.monitorMemory()
	}
	
	return manager
}

// PoolConfig holds configuration for memory pools
type PoolConfig struct {
	Enabled          bool
	EnableMonitoring bool
	StringPoolSize   int64
	BytePoolSize     int64
	MapPoolSize      int64
	GCInterval       time.Duration
	MemoryThreshold  int64
}

// DefaultPoolConfig returns default pool configuration
func DefaultPoolConfig() PoolConfig {
	return PoolConfig{
		Enabled:          true,
		EnableMonitoring: true,
		StringPoolSize:   1024,
		BytePoolSize:     64 * 1024, // 64KB
		MapPoolSize:      100,
		GCInterval:       30 * time.Second,
		MemoryThreshold:  100 * 1024 * 1024, // 100MB
	}
}

// GetStringPool returns the string pool
func (pm *PoolManager) GetStringPool() *StringPool {
	if !pm.enabled {
		return nil
	}
	return pm.stringPool
}

// GetBytePool returns the byte pool
func (pm *PoolManager) GetBytePool() *BytePool {
	if !pm.enabled {
		return nil
	}
	return pm.bytePool
}

// GetMapPool returns the map pool
func (pm *PoolManager) GetMapPool() *MapPool {
	if !pm.enabled {
		return nil
	}
	return pm.mapPool
}

// GetMemoryStats returns current memory statistics
func (pm *PoolManager) GetMemoryStats() MemoryStats {
	pm.mu.RLock()
	defer pm.mu.RUnlock()
	return *pm.stats
}

// monitorMemory continuously monitors memory usage
func (pm *PoolManager) monitorMemory() {
	ticker := time.NewTicker(pm.gcInterval)
	defer ticker.Stop()
	
	for {
		select {
		case <-ticker.C:
			pm.updateMemoryStats()
		case <-pm.done:
			return
		}
	}
}

// updateMemoryStats updates the memory statistics
func (pm *PoolManager) updateMemoryStats() {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	
	pm.mu.Lock()
	pm.stats.Alloc = m.Alloc
	pm.stats.TotalAlloc = m.TotalAlloc
	pm.stats.Sys = m.Sys
	pm.stats.Lookups = m.Lookups
	pm.stats.Mallocs = m.Mallocs
	pm.stats.Frees = m.Frees
	pm.stats.HeapAlloc = m.HeapAlloc
	pm.stats.HeapSys = m.HeapSys
	pm.stats.HeapIdle = m.HeapIdle
	pm.stats.HeapInuse = m.HeapInuse
	pm.stats.HeapReleased = m.HeapReleased
	pm.stats.StackInuse = m.StackInuse
	pm.stats.StackSys = m.StackSys
	pm.stats.GCCPUFraction = m.GCCPUFraction
	pm.stats.NumGC = m.NumGC
	if m.LastGC > 0 {
		pm.stats.LastGC = time.Unix(0, int64(m.LastGC))
	}
	pm.mu.Unlock()
}

// ForceGC forces garbage collection
func (pm *PoolManager) ForceGC() {
	runtime.GC()
	pm.updateMemoryStats()
}

// Close shuts down the pool manager
func (pm *PoolManager) Close() {
	if pm.monitoring {
		close(pm.done)
	}
}

// GetAllStats returns statistics for all pools
func (pm *PoolManager) GetAllStats() map[string]PoolStats {
	stats := make(map[string]PoolStats)
	
	if pm.stringPool != nil {
		stats["string"] = pm.stringPool.Stats()
	}
	if pm.bytePool != nil {
		stats["byte"] = pm.bytePool.Stats()
	}
	if pm.mapPool != nil {
		stats["map"] = pm.mapPool.Stats()
	}
	
	return stats
}

// Global pool manager instance
var globalPoolManager *PoolManager
var globalPoolOnce sync.Once

// GetGlobalPoolManager returns the global pool manager instance
func GetGlobalPoolManager() *PoolManager {
	globalPoolOnce.Do(func() {
		globalPoolManager = NewPoolManager(DefaultPoolConfig())
	})
	return globalPoolManager
}