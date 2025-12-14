package security

import (
	"hash/fnv"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// CacheEntry represents a cached validation result
type CacheEntry struct {
	Result  error
	Expires time.Time
}

// PathValidatorCache provides LRU-style caching for path validation results
type PathValidatorCache struct {
	mu     sync.RWMutex
	cache  map[uint32]*CacheEntry
	maxSize int
	ttl     time.Duration

	// LRU tracking
	accessOrder []uint32
	head, tail  uint32
}

// NewPathValidatorCache creates a new path validation cache
func NewPathValidatorCache(maxSize int, ttl time.Duration) *PathValidatorCache {
	return &PathValidatorCache{
		cache:    make(map[uint32]*CacheEntry, maxSize),
		maxSize:  maxSize,
		ttl:      ttl,
		head:     0,
		tail:     0,
	}
}

// hashPath creates a fast hash of a path for cache lookup
func (pvc *PathValidatorCache) hashPath(path string) uint32 {
	// Use FNV-1a hash for fast distribution
	hasher := fnv.New32a()
	hasher.Write([]byte(filepath.Clean(path)))
	return hasher.Sum32()
}

// Get retrieves a cached validation result
func (pvc *PathValidatorCache) Get(path string) (error, bool) {
	hash := pvc.hashPath(path)

	pvc.mu.RLock()
	defer pvc.mu.RUnlock()

	entry, exists := pvc.cache[hash]
	if !exists {
		return nil, false
	}

	// Check if expired
	if time.Now().After(entry.Expires) {
		return nil, false
	}

	// Move to front in LRU (simplified - could be optimized)
	pvc.moveToFront(hash)

	return entry.Result, true
}

// Set stores a validation result in the cache
func (pvc *PathValidatorCache) Set(path string, result error) {
	hash := pvc.hashPath(path)

	pvc.mu.Lock()
	defer pvc.mu.Unlock()

	// Check if we need to evict
	if len(pvc.cache) >= pvc.maxSize {
		pvc.evictLRU()
	}

	pvc.cache[hash] = &CacheEntry{
		Result:  result,
		Expires: time.Now().Add(pvc.ttl),
	}

	pvc.moveToFront(hash)
}

// evictLRU removes the least recently used item
func (pvc *PathValidatorCache) evictLRU() {
	if len(pvc.cache) == 0 {
		return
	}

	// Simple eviction: remove the first item
	// In a production system, you'd want a proper LRU implementation
	for hash := range pvc.cache {
		delete(pvc.cache, hash)
		break
	}
}

// moveToFront moves an item to the front of the access order
func (pvc *PathValidatorCache) moveToFront(hash uint32) {
	// Simplified LRU - in production, use a proper doubly-linked list
	// For now, we just track that it was accessed
}

// ProtectedPathCache caches protected path prefix checks
type ProtectedPathCache struct {
	mu      sync.RWMutex
	prefixes map[string]bool
	updated  time.Time
	ttl      time.Duration
}

// NewProtectedPathCache creates a new protected path cache
func NewProtectedPathCache(ttl time.Duration) *ProtectedPathCache {
	return &ProtectedPathCache{
		prefixes: make(map[string]bool),
		ttl:      ttl,
	}
}

// CheckPrefix checks if a path has any protected prefix
func (ppc *ProtectedPathCache) CheckPrefix(path string) bool {
	ppc.mu.RLock()
	defer ppc.mu.RUnlock()

	// Check if cache needs refresh
	if time.Since(ppc.updated) > ppc.ttl {
		ppc.mu.RUnlock()
		ppc.refresh()
		ppc.mu.RLock()
	}

	// Check prefixes
	for prefix := range ppc.prefixes {
		if strings.HasPrefix(path, prefix) {
			return true
		}
	}

	return false
}

// refresh updates the protected prefixes list
func (ppc *ProtectedPathCache) refresh() {
	ppc.mu.Lock()
	defer ppc.mu.Unlock()

	// This would be populated from PathValidator.protectedPaths
	// For now, we'll update with common system paths
	ppc.prefixes = map[string]bool{
		"/bin":               true,
		"/sbin":              true,
		"/usr/bin":           true,
		"/usr/sbin":          true,
		"/System":            true,
		"/Library":           true,
		"/etc":               true,
		"/var":               true,
	}
	ppc.updated = time.Now()
}

// ValidateCached wraps PathValidator with caching
func (pv *PathValidator) ValidateCached(path string) error {
	// Initialize cache if needed
	if pv.cache == nil {
		pv.cache = NewPathValidatorCache(10000, 5*time.Minute)
	}

	// Check cache first
	if result, found := pv.cache.Get(path); found {
		return result
	}

	// Perform validation
	err := pv.ValidatePathForDeletion(path)

	// Cache result (cache nil errors as nil for simplicity)
	pv.cache.Set(path, err)

	return err
}

// SkipFileFast performs fast pre-checks before expensive validation
func (pv *PathValidator) SkipFileFast(path string, info interface{}) bool {
	// Quick extensions check
	if strings.HasSuffix(path, ".DS_Store") {
		return true
	}

	// Quick size check if available
	if size, ok := info.(interface{ Size() int64 }); ok {
		if size.Size() > 1024*1024*1024 { // > 1GB
			// Be more careful with large files
			return false
		}
	}

	// Quick path pattern checks
	if strings.Contains(path, "/.git/") {
		return false // Don't skip git files
	}

	return false
}