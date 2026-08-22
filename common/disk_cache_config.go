package common

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
)

// DiskCacheConfig 磁盘缓存配置（由 performance_setting 包更新）
type DiskCacheConfig struct {
	// Enabled 是否启用磁盘缓存
	Enabled bool
	// ThresholdMB 触发磁盘缓存的请求体大小阈值（MB）
	ThresholdMB int
	// MaxSizeMB 磁盘缓存最大总大小（MB）
	MaxSizeMB int
	// Path 磁盘缓存目录
	Path string
}

// 全局磁盘缓存配置
var diskCacheConfig = DiskCacheConfig{
	Enabled:     false,
	ThresholdMB: 10,
	MaxSizeMB:   1024,
	Path:        "",
}
var diskCacheConfigMu sync.RWMutex

// GetDiskCacheConfig 获取磁盘缓存配置
func GetDiskCacheConfig() DiskCacheConfig {
	diskCacheConfigMu.RLock()
	defer diskCacheConfigMu.RUnlock()
	return diskCacheConfig
}

// SetDiskCacheConfig 设置磁盘缓存配置
func SetDiskCacheConfig(config DiskCacheConfig) {
	diskCacheConfigMu.Lock()
	defer diskCacheConfigMu.Unlock()
	config.Path = NormalizeDiskCachePath(config.Path)
	diskCacheConfig = config
}

// IsDiskCacheEnabled 是否启用磁盘缓存
func IsDiskCacheEnabled() bool {
	diskCacheConfigMu.RLock()
	defer diskCacheConfigMu.RUnlock()
	return diskCacheConfig.Enabled
}

// GetDiskCacheThresholdBytes 获取磁盘缓存阈值（字节）
func GetDiskCacheThresholdBytes() int64 {
	diskCacheConfigMu.RLock()
	defer diskCacheConfigMu.RUnlock()
	return int64(diskCacheConfig.ThresholdMB) << 20
}

// GetDiskCacheMaxSizeBytes 获取磁盘缓存最大大小（字节）
func GetDiskCacheMaxSizeBytes() int64 {
	diskCacheConfigMu.RLock()
	defer diskCacheConfigMu.RUnlock()
	return int64(diskCacheConfig.MaxSizeMB) << 20
}

// GetDiskCachePath 获取磁盘缓存目录（已规范化；空表示使用系统临时目录）
func GetDiskCachePath() string {
	diskCacheConfigMu.RLock()
	defer diskCacheConfigMu.RUnlock()
	return diskCacheConfig.Path
}

// NormalizeDiskCachePath returns a usable parent directory for the body cache.
// Empty, foreign-OS absolute paths, and paths that cannot be created fall back to os.TempDir().
// The returned value is the parent path (not including the new-api-body-cache leaf).
func NormalizeDiskCachePath(path string) string {
	path = strings.TrimSpace(path)
	fallback := os.TempDir()
	if path == "" {
		return fallback
	}
	if !isUsableDiskCachePath(path) {
		return fallback
	}
	if err := os.MkdirAll(path, 0755); err != nil {
		return fallback
	}
	return path
}

func isUsableDiskCachePath(path string) bool {
	// Reject obvious cross-OS absolute paths that cannot work on the current host.
	if runtime.GOOS == "windows" {
		// Unix-style absolute path: /Users/... or /tmp/...
		if strings.HasPrefix(path, "/") {
			return false
		}
		// UNC is allowed; drive-relative like C:\ is fine.
		return true
	}
	// Non-Windows: reject Windows drive paths and UNC.
	if len(path) >= 2 && path[1] == ':' {
		return false
	}
	if strings.HasPrefix(path, `\\`) || strings.HasPrefix(path, "//") {
		// UNC on non-Windows is not usable for local cache.
		if !filepath.IsAbs(path) || strings.HasPrefix(path, `\\`) {
			return false
		}
	}
	// On Unix, relative paths are ok; absolute Unix paths are ok.
	return true
}

// DiskCacheStats 磁盘缓存统计信息
type DiskCacheStats struct {
	// 当前活跃的磁盘缓存文件数
	ActiveDiskFiles int64 `json:"active_disk_files"`
	// 当前磁盘缓存总大小（字节）
	CurrentDiskUsageBytes int64 `json:"current_disk_usage_bytes"`
	// 当前内存缓存数量
	ActiveMemoryBuffers int64 `json:"active_memory_buffers"`
	// 当前内存缓存总大小（字节）
	CurrentMemoryUsageBytes int64 `json:"current_memory_usage_bytes"`
	// 磁盘缓存命中次数
	DiskCacheHits int64 `json:"disk_cache_hits"`
	// 内存缓存命中次数
	MemoryCacheHits int64 `json:"memory_cache_hits"`
	// 磁盘缓存最大限制（字节）
	DiskCacheMaxBytes int64 `json:"disk_cache_max_bytes"`
	// 磁盘缓存阈值（字节）
	DiskCacheThresholdBytes int64 `json:"disk_cache_threshold_bytes"`
}

var diskCacheStats DiskCacheStats

// GetDiskCacheStats 获取缓存统计信息
func GetDiskCacheStats() DiskCacheStats {
	stats := DiskCacheStats{
		ActiveDiskFiles:         atomic.LoadInt64(&diskCacheStats.ActiveDiskFiles),
		CurrentDiskUsageBytes:   atomic.LoadInt64(&diskCacheStats.CurrentDiskUsageBytes),
		ActiveMemoryBuffers:     atomic.LoadInt64(&diskCacheStats.ActiveMemoryBuffers),
		CurrentMemoryUsageBytes: atomic.LoadInt64(&diskCacheStats.CurrentMemoryUsageBytes),
		DiskCacheHits:           atomic.LoadInt64(&diskCacheStats.DiskCacheHits),
		MemoryCacheHits:         atomic.LoadInt64(&diskCacheStats.MemoryCacheHits),
		DiskCacheMaxBytes:       GetDiskCacheMaxSizeBytes(),
		DiskCacheThresholdBytes: GetDiskCacheThresholdBytes(),
	}
	return stats
}

// IncrementDiskFiles 增加磁盘文件计数
func IncrementDiskFiles(size int64) {
	atomic.AddInt64(&diskCacheStats.ActiveDiskFiles, 1)
	atomic.AddInt64(&diskCacheStats.CurrentDiskUsageBytes, size)
}

// DecrementDiskFiles 减少磁盘文件计数
func DecrementDiskFiles(size int64) {
	if atomic.AddInt64(&diskCacheStats.ActiveDiskFiles, -1) < 0 {
		atomic.StoreInt64(&diskCacheStats.ActiveDiskFiles, 0)
	}
	if atomic.AddInt64(&diskCacheStats.CurrentDiskUsageBytes, -size) < 0 {
		atomic.StoreInt64(&diskCacheStats.CurrentDiskUsageBytes, 0)
	}
}

// IncrementMemoryBuffers 增加内存缓存计数
func IncrementMemoryBuffers(size int64) {
	atomic.AddInt64(&diskCacheStats.ActiveMemoryBuffers, 1)
	atomic.AddInt64(&diskCacheStats.CurrentMemoryUsageBytes, size)
}

// DecrementMemoryBuffers 减少内存缓存计数
func DecrementMemoryBuffers(size int64) {
	atomic.AddInt64(&diskCacheStats.ActiveMemoryBuffers, -1)
	atomic.AddInt64(&diskCacheStats.CurrentMemoryUsageBytes, -size)
}

// IncrementDiskCacheHits 增加磁盘缓存命中次数
func IncrementDiskCacheHits() {
	atomic.AddInt64(&diskCacheStats.DiskCacheHits, 1)
}

// IncrementMemoryCacheHits 增加内存缓存命中次数
func IncrementMemoryCacheHits() {
	atomic.AddInt64(&diskCacheStats.MemoryCacheHits, 1)
}

// ResetDiskCacheStats 重置命中统计信息（不重置当前使用量）
func ResetDiskCacheStats() {
	atomic.StoreInt64(&diskCacheStats.DiskCacheHits, 0)
	atomic.StoreInt64(&diskCacheStats.MemoryCacheHits, 0)
}

// ResetDiskCacheUsage 重置磁盘缓存使用量统计（用于清理缓存后）
func ResetDiskCacheUsage() {
	atomic.StoreInt64(&diskCacheStats.ActiveDiskFiles, 0)
	atomic.StoreInt64(&diskCacheStats.CurrentDiskUsageBytes, 0)
}

// SyncDiskCacheStats 从实际磁盘状态同步统计信息
// 用于修正统计与实际不符的情况
func SyncDiskCacheStats() {
	fileCount, totalSize, err := GetDiskCacheInfo()
	if err != nil {
		return
	}
	atomic.StoreInt64(&diskCacheStats.ActiveDiskFiles, int64(fileCount))
	atomic.StoreInt64(&diskCacheStats.CurrentDiskUsageBytes, totalSize)
}

// IsDiskCacheAvailable 检查是否可以创建新的磁盘缓存
func IsDiskCacheAvailable(requestSize int64) bool {
	if !IsDiskCacheEnabled() {
		return false
	}
	maxBytes := GetDiskCacheMaxSizeBytes()
	currentUsage := atomic.LoadInt64(&diskCacheStats.CurrentDiskUsageBytes)
	return currentUsage+requestSize <= maxBytes
}