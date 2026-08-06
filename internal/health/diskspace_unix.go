//go:build !windows

package health

import (
	"fmt"
	"path/filepath"
	"syscall"
)

// CheckDiskSpace checks available disk space for a given path
func CheckDiskSpace(path string) (*DiskSpaceInfo, error) {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return nil, fmt.Errorf("failed to get absolute path: %w", err)
	}

	var stat syscall.Statfs_t
	if err := syscall.Statfs(filepath.Dir(absPath), &stat); err != nil {
		return nil, fmt.Errorf("failed to get disk space info: %w", err)
	}

	total := uint64(stat.Blocks) * uint64(stat.Bsize)
	free := uint64(stat.Bavail) * uint64(stat.Bsize)
	used := total - free
	usedPercent := (float64(used) / float64(total)) * 100.0
	freePercent := 100.0 - usedPercent

	return &DiskSpaceInfo{
		TotalBytes:    total,
		FreeBytes:     free,
		UsedBytes:     used,
		UsedPercent:   usedPercent,
		FreePercent:   freePercent,
		AlertNeeded:   freePercent < 10.0,
		CriticalLevel: freePercent < 5.0,
	}, nil
}
