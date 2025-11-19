package cleaner

import (
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"syscall"
)

// PermissionManager handles permission checking
type PermissionManager struct {
	isRoot bool
}

// NewPermissionManager creates a new PermissionManager
func NewPermissionManager() *PermissionManager {
	currentUser, _ := user.Current()
	isRoot := currentUser != nil && currentUser.Uid == "0"

	return &PermissionManager{
		isRoot: isRoot,
	}
}

// IsRunningAsRoot checks if the current process is running as root
func (pm *PermissionManager) IsRunningAsRoot() bool {
	return pm.isRoot
}

// CanDelete checks if we have permission to delete a file
func (pm *PermissionManager) CanDelete(path string) (bool, error) {
	// If running as root, we can delete anything
	if pm.isRoot {
		return true, nil
	}

	// Check if file exists
	if _, err := os.Stat(path); err != nil {
		return false, err
	}

	// Check if we can write to the parent directory
	parentDir := filepath.Dir(path)
	parentInfo, err := os.Stat(parentDir)
	if err != nil {
		return false, err
	}

	// Get file stat
	stat, ok := parentInfo.Sys().(*syscall.Stat_t)
	if !ok {
		return false, fmt.Errorf("unable to get file stats")
	}

	currentUser, err := user.Current()
	if err != nil {
		return false, err
	}

	uid := currentUser.Uid
	gid := currentUser.Gid

	// Check if user owns the parent directory
	if fmt.Sprint(stat.Uid) == uid {
		// Check if owner has write permission
		return parentInfo.Mode()&0200 != 0, nil
	}

	// Check group permissions
	if fmt.Sprint(stat.Gid) == gid {
		return parentInfo.Mode()&0020 != 0, nil
	}

	// Check other permissions
	return parentInfo.Mode()&0002 != 0, nil
}

// RequiresElevation checks if a path requires elevated permissions to delete
func (pm *PermissionManager) RequiresElevation(path string) bool {
	if pm.isRoot {
		return false // Already running as root
	}

	canDelete, err := pm.CanDelete(path)
	if err != nil {
		// If we can't even check, assume it needs elevation
		return true
	}

	return !canDelete
}

// IsSpecialFile checks if a path is a special file (device, socket, pipe)
func IsSpecialFile(path string) (bool, error) {
	info, err := os.Lstat(path) // Use Lstat to not follow symlinks
	if err != nil {
		return false, err
	}

	mode := info.Mode()

	// Check for special file types
	switch {
	case mode&os.ModeDevice != 0:
		return true, fmt.Errorf("is a device file")
	case mode&os.ModeCharDevice != 0:
		return true, fmt.Errorf("is a character device")
	case mode&os.ModeSocket != 0:
		return true, fmt.Errorf("is a socket")
	case mode&os.ModeNamedPipe != 0:
		return true, fmt.Errorf("is a named pipe (FIFO)")
	case mode&os.ModeSymlink != 0:
		// For symlinks, check what they point to
		target, err := os.Readlink(path)
		if err != nil {
			return false, err
		}
		// Resolve to absolute path
		if !filepath.IsAbs(target) {
			target = filepath.Join(filepath.Dir(path), target)
		}
		// Recursively check the target
		return IsSpecialFile(target)
	}

	return false, nil
}

// IsSafeToDelete performs comprehensive safety checks on a file
func IsSafeToDelete(path string) error {
	// Check if it's a special file
	if isSpecial, err := IsSpecialFile(path); isSpecial {
		return fmt.Errorf("refusing to delete special file: %w", err)
	}

	// Check if file exists
	if _, err := os.Lstat(path); err != nil {
		return err
	}

	return nil
}

// PermissionReport contains analysis of files by permission requirements
type PermissionReport struct {
	NormalFiles      []string
	RequiresSudo     []string
	InaccessibleFiles map[string]error
	TotalNormalSize  int64
	TotalSudoSize    int64
}

// AnalyzePermissions analyzes a list of file paths and categorizes them by permission requirements
func (pm *PermissionManager) AnalyzePermissions(files []string, getSizeFunc func(string) int64) *PermissionReport {
	report := &PermissionReport{
		NormalFiles:       []string{},
		RequiresSudo:      []string{},
		InaccessibleFiles: make(map[string]error),
	}

	for _, path := range files {
		// Check if file still exists
		info, err := os.Stat(path)
		if err != nil {
			if os.IsNotExist(err) {
				// File already deleted, skip
				continue
			}
			// Can't access file at all
			report.InaccessibleFiles[path] = err
			continue
		}

		size := info.Size()
		if getSizeFunc != nil {
			size = getSizeFunc(path)
		}

		// Check if we can delete it
		canDelete, err := pm.CanDelete(path)
		if err != nil {
			report.InaccessibleFiles[path] = err
			continue
		}

		if canDelete {
			report.NormalFiles = append(report.NormalFiles, path)
			report.TotalNormalSize += size
		} else {
			report.RequiresSudo = append(report.RequiresSudo, path)
			report.TotalSudoSize += size
		}
	}

	return report
}
