package cleaner

import (
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
)

// PermissionManager handles permission checking with enhanced capabilities
type PermissionManager struct {
	isRoot       bool
	currentUID   uint32
	currentGID   uint32
	userGroups   []uint32
	userName     string
}

// NewPermissionManager creates a new PermissionManager with comprehensive user info
func NewPermissionManager() *PermissionManager {
	pm := &PermissionManager{}

	currentUser, err := user.Current()
	if err == nil {
		pm.userName = currentUser.Username

		// Parse UID
		if uid, err := strconv.ParseUint(currentUser.Uid, 10, 32); err == nil {
			pm.currentUID = uint32(uid)
			pm.isRoot = uid == 0
		}

		// Parse primary GID
		if gid, err := strconv.ParseUint(currentUser.Gid, 10, 32); err == nil {
			pm.currentGID = uint32(gid)
			pm.userGroups = append(pm.userGroups, uint32(gid))
		}

		// Get all supplementary groups
		if groups, err := currentUser.GroupIds(); err == nil {
			for _, gidStr := range groups {
				if gid, err := strconv.ParseUint(gidStr, 10, 32); err == nil {
					// Avoid duplicates
					found := false
					for _, existing := range pm.userGroups {
						if existing == uint32(gid) {
							found = true
							break
						}
					}
					if !found {
						pm.userGroups = append(pm.userGroups, uint32(gid))
					}
				}
			}
		}
	}

	return pm
}

// IsRunningAsRoot checks if the current process is running as root
func (pm *PermissionManager) IsRunningAsRoot() bool {
	return pm.isRoot
}

// GetUserInfo returns current user information for debugging
func (pm *PermissionManager) GetUserInfo() string {
	return fmt.Sprintf("user=%s uid=%d gid=%d groups=%v root=%v",
		pm.userName, pm.currentUID, pm.currentGID, pm.userGroups, pm.isRoot)
}

// FilePermissionInfo contains detailed permission information for a file
type FilePermissionInfo struct {
	Path            string
	Exists          bool
	IsDir           bool
	IsSymlink       bool
	SymlinkTarget   string
	FileUID         uint32
	FileGID         uint32
	FileMode        os.FileMode
	ParentWritable  bool
	FileWritable    bool
	CanDelete       bool
	Reason          string
	RequiresSudo    bool
	IsSpecialFile   bool
	SpecialFileType string
}

// AnalyzeFilePermissions provides detailed permission analysis for a single file
func (pm *PermissionManager) AnalyzeFilePermissions(path string) *FilePermissionInfo {
	info := &FilePermissionInfo{
		Path: path,
	}

	// Use Lstat to not follow symlinks
	fileInfo, err := os.Lstat(path)
	if err != nil {
		if os.IsNotExist(err) {
			info.Exists = false
			info.Reason = "file does not exist"
		} else if os.IsPermission(err) {
			info.Exists = true // Probably exists but can't access
			info.Reason = fmt.Sprintf("permission denied: %v", err)
			info.RequiresSudo = true
		} else {
			info.Reason = fmt.Sprintf("error accessing file: %v", err)
		}
		return info
	}

	info.Exists = true
	info.FileMode = fileInfo.Mode()
	info.IsDir = fileInfo.IsDir()

	// Check for symlink
	if fileInfo.Mode()&os.ModeSymlink != 0 {
		info.IsSymlink = true
		if target, err := os.Readlink(path); err == nil {
			info.SymlinkTarget = target
		}
	}

	// Check for special files
	special, specialType := pm.checkSpecialFile(fileInfo.Mode())
	info.IsSpecialFile = special
	info.SpecialFileType = specialType

	// Get syscall stat for UID/GID
	if stat, ok := fileInfo.Sys().(*syscall.Stat_t); ok {
		info.FileUID = stat.Uid
		info.FileGID = stat.Gid
	}

	// Check parent directory write permissions
	parentDir := filepath.Dir(path)
	parentWritable, parentReason := pm.checkDirectoryWritable(parentDir)
	info.ParentWritable = parentWritable

	// Check file write permissions (needed for some operations)
	info.FileWritable = pm.checkFileWritable(fileInfo, path)

	// Determine if we can delete
	if pm.isRoot {
		info.CanDelete = true
		info.Reason = "running as root"
	} else if special {
		info.CanDelete = false
		info.Reason = fmt.Sprintf("special file type: %s", specialType)
	} else if !parentWritable {
		info.CanDelete = false
		info.RequiresSudo = true
		info.Reason = parentReason
	} else {
		info.CanDelete = true
		info.Reason = "user has delete permission"
	}

	return info
}

// checkSpecialFile checks if the file mode indicates a special file
func (pm *PermissionManager) checkSpecialFile(mode os.FileMode) (bool, string) {
	switch {
	case mode&os.ModeDevice != 0:
		return true, "block device"
	case mode&os.ModeCharDevice != 0:
		return true, "character device"
	case mode&os.ModeSocket != 0:
		return true, "socket"
	case mode&os.ModeNamedPipe != 0:
		return true, "named pipe (FIFO)"
	case mode&os.ModeSetuid != 0:
		return true, "setuid file"
	case mode&os.ModeSetgid != 0:
		return true, "setgid file"
	}
	return false, ""
}

// checkDirectoryWritable checks if the current user can write to a directory
func (pm *PermissionManager) checkDirectoryWritable(dirPath string) (bool, string) {
	dirInfo, err := os.Stat(dirPath)
	if err != nil {
		return false, fmt.Sprintf("cannot access parent directory: %v", err)
	}

	if !dirInfo.IsDir() {
		return false, "parent path is not a directory"
	}

	stat, ok := dirInfo.Sys().(*syscall.Stat_t)
	if !ok {
		return false, "cannot get directory ownership info"
	}

	mode := dirInfo.Mode()

	// Check sticky bit - if set, only owner can delete files
	stickyBit := mode&os.ModeSticky != 0

	// Owner check
	if stat.Uid == pm.currentUID {
		if mode&0200 != 0 { // Owner write permission
			return true, ""
		}
		return false, "owner does not have write permission on parent directory"
	}

	// Group check
	for _, gid := range pm.userGroups {
		if stat.Gid == gid {
			if mode&0020 != 0 { // Group write permission
				if stickyBit {
					return false, "directory has sticky bit set, only owner can delete"
				}
				return true, ""
			}
		}
	}

	// Others check
	if mode&0002 != 0 { // Others write permission
		if stickyBit {
			return false, "directory has sticky bit set, only owner can delete"
		}
		return true, ""
	}

	return false, "no write permission on parent directory"
}

// checkFileWritable checks if the current user can write to a file
func (pm *PermissionManager) checkFileWritable(info os.FileInfo, path string) bool {
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return false
	}

	mode := info.Mode()

	// Owner check
	if stat.Uid == pm.currentUID {
		return mode&0200 != 0
	}

	// Group check
	for _, gid := range pm.userGroups {
		if stat.Gid == gid {
			return mode&0020 != 0
		}
	}

	// Others check
	return mode&0002 != 0
}

// CanDelete checks if we have permission to delete a file
func (pm *PermissionManager) CanDelete(path string) (bool, error) {
	info := pm.AnalyzeFilePermissions(path)

	if !info.Exists {
		return false, fmt.Errorf("file does not exist")
	}

	if info.IsSpecialFile {
		return false, fmt.Errorf("refusing to delete special file: %s", info.SpecialFileType)
	}

	return info.CanDelete, nil
}

// RequiresElevation checks if a path requires elevated permissions to delete
func (pm *PermissionManager) RequiresElevation(path string) bool {
	if pm.isRoot {
		return false
	}

	info := pm.AnalyzeFilePermissions(path)
	return info.RequiresSudo
}

// IsSpecialFile checks if a path is a special file (device, socket, pipe, setuid/setgid)
func IsSpecialFile(path string) (bool, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return false, err
	}

	mode := info.Mode()

	// Check for special file types that should never be deleted
	switch {
	case mode&os.ModeDevice != 0:
		return true, fmt.Errorf("is a block device file")
	case mode&os.ModeCharDevice != 0:
		return true, fmt.Errorf("is a character device")
	case mode&os.ModeSocket != 0:
		return true, fmt.Errorf("is a socket")
	case mode&os.ModeNamedPipe != 0:
		return true, fmt.Errorf("is a named pipe (FIFO)")
	case mode&os.ModeSetuid != 0:
		return true, fmt.Errorf("is a setuid file - potentially dangerous")
	case mode&os.ModeSetgid != 0:
		return true, fmt.Errorf("is a setgid file - potentially dangerous")
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
		// Check if target is special (but limit recursion)
		return isSpecialFileNonRecursive(target)
	}

	return false, nil
}

// isSpecialFileNonRecursive checks for special file without following further symlinks
func isSpecialFileNonRecursive(path string) (bool, error) {
	info, err := os.Lstat(path)
	if err != nil {
		// If we can't stat the target, it's probably a broken symlink - safe to delete the link
		return false, nil
	}

	mode := info.Mode()

	switch {
	case mode&os.ModeDevice != 0:
		return true, fmt.Errorf("symlink points to a block device")
	case mode&os.ModeCharDevice != 0:
		return true, fmt.Errorf("symlink points to a character device")
	case mode&os.ModeSocket != 0:
		return true, fmt.Errorf("symlink points to a socket")
	case mode&os.ModeNamedPipe != 0:
		return true, fmt.Errorf("symlink points to a named pipe")
	}

	return false, nil
}

// IsSafeToDelete performs comprehensive safety checks on a file
func IsSafeToDelete(path string) error {
	// Check if it's a special file
	if isSpecial, err := IsSpecialFile(path); isSpecial {
		return fmt.Errorf("refusing to delete special file: %w", err)
	}

	// Check if path contains dangerous patterns
	cleanPath := filepath.Clean(path)
	if strings.Contains(cleanPath, "..") {
		return fmt.Errorf("path contains directory traversal")
	}

	// Verify file exists and get info
	_, err := os.Lstat(path)
	if err != nil {
		return err
	}

	// Note: Directories are allowed here because the cleaner handles them
	// appropriately with os.RemoveAll() for dev artifacts like node_modules,
	// venv, and build directories.

	return nil
}

// PermissionReport contains analysis of files by permission requirements
type PermissionReport struct {
	NormalFiles       []string
	RequiresSudo      []string
	InaccessibleFiles map[string]error
	SpecialFiles      map[string]string // path -> special file type
	TotalNormalSize   int64
	TotalSudoSize     int64
	AnalysisDetails   []*FilePermissionInfo
}

// AnalyzePermissions analyzes a list of file paths and categorizes them by permission requirements
func (pm *PermissionManager) AnalyzePermissions(files []string, getSizeFunc func(string) int64) *PermissionReport {
	report := &PermissionReport{
		NormalFiles:       make([]string, 0, len(files)/2),
		RequiresSudo:      make([]string, 0),
		InaccessibleFiles: make(map[string]error),
		SpecialFiles:      make(map[string]string),
		AnalysisDetails:   make([]*FilePermissionInfo, 0),
	}

	for _, path := range files {
		info := pm.AnalyzeFilePermissions(path)
		report.AnalysisDetails = append(report.AnalysisDetails, info)

		if !info.Exists {
			// File already deleted, skip silently
			continue
		}

		// Get size
		var size int64
		if getSizeFunc != nil {
			size = getSizeFunc(path)
		} else if fileInfo, err := os.Stat(path); err == nil {
			size = fileInfo.Size()
		}

		// Categorize
		if info.IsSpecialFile {
			report.SpecialFiles[path] = info.SpecialFileType
			continue
		}

		if info.CanDelete {
			report.NormalFiles = append(report.NormalFiles, path)
			report.TotalNormalSize += size
		} else if info.RequiresSudo {
			report.RequiresSudo = append(report.RequiresSudo, path)
			report.TotalSudoSize += size
		} else {
			report.InaccessibleFiles[path] = fmt.Errorf("%s", info.Reason)
		}
	}

	return report
}

// VerifyDeletionSafe performs pre-deletion verification
// Returns error if the file state has changed since analysis
func VerifyDeletionSafe(path string, expectedInode uint64, expectedSize int64) error {
	info, err := os.Lstat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // Already deleted, that's fine
		}
		return fmt.Errorf("cannot verify file state: %w", err)
	}

	// Verify inode hasn't changed (TOCTOU protection)
	if stat, ok := info.Sys().(*syscall.Stat_t); ok {
		if expectedInode > 0 && stat.Ino != expectedInode {
			return fmt.Errorf("file inode changed: expected %d, got %d (possible race condition)", expectedInode, stat.Ino)
		}
	}

	// Verify size hasn't changed dramatically (could indicate different file)
	if expectedSize > 0 && info.Size() != expectedSize {
		// Allow small differences for actively used files
		diff := info.Size() - expectedSize
		if diff < 0 {
			diff = -diff
		}
		// If size changed by more than 10%, something might be wrong
		if float64(diff)/float64(expectedSize) > 0.1 {
			return fmt.Errorf("file size changed significantly: expected %d, got %d", expectedSize, info.Size())
		}
	}

	// Verify it's still a regular file (not replaced with symlink/device)
	if info.Mode()&(os.ModeDevice|os.ModeCharDevice|os.ModeSocket|os.ModeNamedPipe) != 0 {
		return fmt.Errorf("file type changed to special file")
	}

	return nil
}

// GetFileInode returns the inode number for a file (for TOCTOU protection)
func GetFileInode(path string) (uint64, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return 0, err
	}

	if stat, ok := info.Sys().(*syscall.Stat_t); ok {
		return stat.Ino, nil
	}

	return 0, fmt.Errorf("cannot get inode for file")
}
