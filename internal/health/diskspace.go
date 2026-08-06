package health

import (
	"fmt"
	"os"
	"path/filepath"

	"go.uber.org/zap"
)

// DiskSpaceInfo contains disk space information
type DiskSpaceInfo struct {
	TotalBytes    uint64
	FreeBytes     uint64
	UsedBytes     uint64
	UsedPercent   float64
	FreePercent   float64
	AlertNeeded   bool
	CriticalLevel bool
}

// CheckDiskSpaceBeforeWrite checks disk space before writing a file and returns error if insufficient
func CheckDiskSpaceBeforeWrite(path string, requiredBytes uint64, logger *zap.Logger) error {
	info, err := CheckDiskSpace(path)
	if err != nil {
		logger.Warn("failed to check disk space",
			zap.String("path", path),
			zap.Error(err))
		return nil // Don't fail, just log
	}

	// Log warnings based on disk space levels
	if info.CriticalLevel {
		logger.Error("critical disk space level",
			zap.String("path", path),
			zap.Float64("free_percent", info.FreePercent),
			zap.Uint64("free_bytes", info.FreeBytes),
			zap.String("available", formatBytes(info.FreeBytes)))
	} else if info.AlertNeeded {
		logger.Warn("low disk space",
			zap.String("path", path),
			zap.Float64("free_percent", info.FreePercent),
			zap.Uint64("free_bytes", info.FreeBytes),
			zap.String("available", formatBytes(info.FreeBytes)))
	}

	// Check if we have enough space for the write operation
	if requiredBytes > 0 && info.FreeBytes < requiredBytes {
		return fmt.Errorf("insufficient disk space: required %s, available %s",
			formatBytes(requiredBytes),
			formatBytes(info.FreeBytes))
	}

	// Don't proceed if disk is critically low
	if info.CriticalLevel && info.FreeBytes < requiredBytes*2 {
		return fmt.Errorf("disk space critically low, refusing write operation: %s available",
			formatBytes(info.FreeBytes))
	}

	return nil
}

// CheckAndCreateDir checks disk space before creating a directory
func CheckAndCreateDir(path string, logger *zap.Logger) error {
	if err := CheckDiskSpaceBeforeWrite(path, 0, logger); err != nil {
		return err
	}
	return os.MkdirAll(filepath.Dir(path), 0755)
}

// formatBytes formats bytes into human-readable format
func formatBytes(bytes uint64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := uint64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}
