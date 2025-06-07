package monitoring

import (
	"crypto/md5"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"time"
)

// CacheManager handles intelligent caching for web pages
type CacheManager struct {
	cacheDir string
	ttl      time.Duration
	enabled  bool
}

// CacheEntry represents a cached page with metadata
type CacheEntry struct {
	URL         string            `json:"url"`
	Content     string            `json:"content"`
	Headers     map[string]string `json:"headers"`
	CachedAt    time.Time         `json:"cached_at"`
	ContentHash string            `json:"content_hash"`
	Size        int               `json:"size"`
}

func NewCacheManager(cacheDir string, ttl time.Duration, enabled bool) *CacheManager {
	if enabled && cacheDir != "" {
		// Create cache directory if it doesn't exist
		os.MkdirAll(cacheDir, 0755)
	}
	
	return &CacheManager{
		cacheDir: cacheDir,
		ttl:      ttl,
		enabled:  enabled,
	}
}

// GenerateCacheKey creates a cache key from URL
func (cm *CacheManager) GenerateCacheKey(url string) string {
	hash := md5.Sum([]byte(url))
	return fmt.Sprintf("%x", hash)
}

// GetCachePath returns the full path to cache file
func (cm *CacheManager) GetCachePath(url string) string {
	if !cm.enabled || cm.cacheDir == "" {
		return ""
	}
	key := cm.GenerateCacheKey(url)
	return filepath.Join(cm.cacheDir, key+".json")
}

// IsCached checks if URL is cached and not expired
func (cm *CacheManager) IsCached(url string) bool {
	if !cm.enabled {
		return false
	}
	
	cachePath := cm.GetCachePath(url)
	if cachePath == "" {
		return false
	}
	
	// Check if file exists
	if _, err := os.Stat(cachePath); os.IsNotExist(err) {
		return false
	}
	
	// Check if cache is expired
	entry, err := cm.GetCacheEntry(url)
	if err != nil {
		return false
	}
	
	return time.Since(entry.CachedAt) < cm.ttl
}

// GetCacheEntry retrieves cached content
func (cm *CacheManager) GetCacheEntry(url string) (*CacheEntry, error) {
	if !cm.enabled {
		return nil, fmt.Errorf("cache disabled")
	}
	
	cachePath := cm.GetCachePath(url)
	if cachePath == "" {
		return nil, fmt.Errorf("invalid cache path")
	}
	
	data, err := os.ReadFile(cachePath)
	if err != nil {
		return nil, err
	}
	
	var entry CacheEntry
	err = json.Unmarshal(data, &entry)
	if err != nil {
		return nil, err
	}
	
	return &entry, nil
}

// SaveToCache stores content in cache
func (cm *CacheManager) SaveToCache(url, content string, headers map[string]string) error {
	if !cm.enabled {
		return nil // Silently skip if disabled
	}
	
	cachePath := cm.GetCachePath(url)
	if cachePath == "" {
		return fmt.Errorf("invalid cache path")
	}
	
	// Generate content hash for change detection
	contentHash := fmt.Sprintf("%x", md5.Sum([]byte(content)))
	
	entry := CacheEntry{
		URL:         url,
		Content:     content,
		Headers:     headers,
		CachedAt:    time.Now(),
		ContentHash: contentHash,
		Size:        len(content),
	}
	
	data, err := json.MarshalIndent(entry, "", "  ")
	if err != nil {
		return err
	}
	
	return os.WriteFile(cachePath, data, 0644)
}

// GetCacheStats returns cache statistics
func (cm *CacheManager) GetCacheStats() map[string]interface{} {
	if !cm.enabled || cm.cacheDir == "" {
		return map[string]interface{}{
			"enabled": false,
		}
	}
	
	stats := map[string]interface{}{
		"enabled":      true,
		"cache_dir":    cm.cacheDir,
		"ttl_hours":    cm.ttl.Hours(),
		"total_files":  0,
		"total_size":   int64(0),
		"expired_files": 0,
	}
	
	// Walk through cache directory
	filepath.WalkDir(cm.cacheDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || filepath.Ext(path) != ".json" {
			return nil
		}
		
		// Count files and size
		if info, err := d.Info(); err == nil {
			stats["total_files"] = stats["total_files"].(int) + 1
			stats["total_size"] = stats["total_size"].(int64) + info.Size()
			
			// Check if expired
			if time.Since(info.ModTime()) > cm.ttl {
				stats["expired_files"] = stats["expired_files"].(int) + 1
			}
		}
		
		return nil
	})
	
	return stats
}

// CleanExpiredCache removes expired cache entries
func (cm *CacheManager) CleanExpiredCache() error {
	if !cm.enabled || cm.cacheDir == "" {
		return nil
	}
	
	cleaned := 0
	err := filepath.WalkDir(cm.cacheDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || filepath.Ext(path) != ".json" {
			return nil
		}
		
		if info, err := d.Info(); err == nil {
			if time.Since(info.ModTime()) > cm.ttl {
				if err := os.Remove(path); err == nil {
					cleaned++
				}
			}
		}
		
		return nil
	})
	
	if cleaned > 0 {
		fmt.Printf("🧹 Cleaned %d expired cache entries\n", cleaned)
	}
	
	return err
}