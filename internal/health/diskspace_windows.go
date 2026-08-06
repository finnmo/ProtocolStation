//go:build windows

package health

import (
	"fmt"
	"path/filepath"
	"syscall"
	"unsafe"
)

var (
	kernel32           = syscall.NewLazyDLL("kernel32.dll")
	getDiskFreeSpaceEx = kernel32.NewProc("GetDiskFreeSpaceExW")
)

// CheckDiskSpace checks available disk space for a given path
func CheckDiskSpace(path string) (*DiskSpaceInfo, error) {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return nil, fmt.Errorf("failed to get absolute path: %w", err)
	}

	dirPtr, err := syscall.UTF16PtrFromString(filepath.Dir(absPath))
	if err != nil {
		return nil, fmt.Errorf("failed to encode path: %w", err)
	}

	var freeBytesAvailable, totalBytes, totalFreeBytes uint64
	ret, _, callErr := getDiskFreeSpaceEx.Call(
		uintptr(unsafe.Pointer(dirPtr)),
		uintptr(unsafe.Pointer(&freeBytesAvailable)),
		uintptr(unsafe.Pointer(&totalBytes)),
		uintptr(unsafe.Pointer(&totalFreeBytes)),
	)
	if ret == 0 {
		return nil, fmt.Errorf("GetDiskFreeSpaceEx failed: %w", callErr)
	}

	total := totalBytes
	free := freeBytesAvailable
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
