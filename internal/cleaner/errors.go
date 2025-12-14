package cleaner

import (
	"errors"
	"fmt"
	"os"
	"syscall"
)

// ErrorReason categorizes why a deletion failed
type ErrorReason int

const (
	ErrorPermissionDenied ErrorReason = iota
	ErrorFileInUse
	ErrorFileNotFound
	ErrorIsDirectory
	ErrorInvalidPath
	ErrorUnknown
)

// String returns a human-readable error reason
func (e ErrorReason) String() string {
	switch e {
	case ErrorPermissionDenied:
		return "Permission denied"
	case ErrorFileInUse:
		return "File is in use"
	case ErrorFileNotFound:
		return "File not found"
	case ErrorIsDirectory:
		return "Is a directory"
	case ErrorInvalidPath:
		return "Invalid path"
	case ErrorUnknown:
		return "Unknown error"
	default:
		return "Unspecified error"
	}
}

// DeletionError represents a detailed deletion error
type DeletionError struct {
	Path      string
	Reason    ErrorReason
	Original  error
	Retryable bool
	NeedsSudo bool
}

// Error implements the error interface
func (e *DeletionError) Error() string {
	return fmt.Sprintf("%s: %s (%v)", e.Path, e.Reason, e.Original)
}

// UserMessage returns a user-friendly error message
func (e *DeletionError) UserMessage() string {
	switch e.Reason {
	case ErrorPermissionDenied:
		if e.NeedsSudo {
			return fmt.Sprintf("⚠️  Need elevated permissions to delete: %s", e.Path)
		}
		return fmt.Sprintf("⚠️  Permission denied: %s", e.Path)
	case ErrorFileInUse:
		return fmt.Sprintf("⚠️  File is being used: %s (close the application and try again)", e.Path)
	case ErrorFileNotFound:
		return fmt.Sprintf("ℹ️  Already deleted: %s", e.Path)
	case ErrorIsDirectory:
		return fmt.Sprintf("⚠️  Cannot delete directory: %s (use recursive delete)", e.Path)
	case ErrorInvalidPath:
		return fmt.Sprintf("❌ Invalid or unsafe path: %s", e.Path)
	default:
		return fmt.Sprintf("❌ Error deleting %s: %v", e.Path, e.Original)
	}
}

// CategorizeError analyzes an error and returns a categorized DeletionError
func CategorizeError(path string, err error) *DeletionError {
	if err == nil {
		return nil
	}

	delErr := &DeletionError{
		Path:     path,
		Original: err,
		Reason:   ErrorUnknown,
	}

	// Check if file not found
	if os.IsNotExist(err) {
		delErr.Reason = ErrorFileNotFound
		delErr.Retryable = false
		delErr.NeedsSudo = false
		return delErr
	}

	// Check if permission error
	if os.IsPermission(err) {
		delErr.Reason = ErrorPermissionDenied
		delErr.Retryable = false
		delErr.NeedsSudo = true
		return delErr
	}

	// Check syscall errors
	var errno syscall.Errno
	if errors.As(err, &errno) {
		switch errno {
		case syscall.EACCES, syscall.EPERM:
			delErr.Reason = ErrorPermissionDenied
			delErr.Retryable = false
			delErr.NeedsSudo = true
		case syscall.EBUSY, syscall.ETXTBSY:
			delErr.Reason = ErrorFileInUse
			delErr.Retryable = true
			delErr.NeedsSudo = false
		case syscall.ENOENT:
			delErr.Reason = ErrorFileNotFound
			delErr.Retryable = false
			delErr.NeedsSudo = false
		case syscall.EISDIR:
			delErr.Reason = ErrorIsDirectory
			delErr.Retryable = false
			delErr.NeedsSudo = false
		default:
			delErr.Reason = ErrorUnknown
			delErr.Retryable = false
		}
		return delErr
	}

	// Default to unknown
	delErr.Retryable = false
	return delErr
}

// GroupErrors groups deletion errors by reason
func GroupErrors(errors []*DeletionError) map[ErrorReason][]*DeletionError {
	grouped := make(map[ErrorReason][]*DeletionError)
	for _, err := range errors {
		grouped[err.Reason] = append(grouped[err.Reason], err)
	}
	return grouped
}

// FormatErrorSummary creates a user-friendly summary of errors
func FormatErrorSummary(errors []*DeletionError) string {
	if len(errors) == 0 {
		return ""
	}

	grouped := GroupErrors(errors)
	summary := fmt.Sprintf("\n⚠️  Issues encountered:\n")

	// Permission denied
	if perms, ok := grouped[ErrorPermissionDenied]; ok {
		summary += fmt.Sprintf("   ├─ Permission denied: %d files\n", len(perms))
		summary += "   │  └─ Tip: Run with sudo or elevate permissions\n"
	}

	// File in use
	if busy, ok := grouped[ErrorFileInUse]; ok {
		summary += fmt.Sprintf("   ├─ File in use: %d files\n", len(busy))
		summary += "   │  └─ Tip: Close applications and retry\n"
	}

	// File not found
	if notFound, ok := grouped[ErrorFileNotFound]; ok {
		summary += fmt.Sprintf("   ├─ Already deleted: %d files\n", len(notFound))
	}

	// Directories
	if dirs, ok := grouped[ErrorIsDirectory]; ok {
		summary += fmt.Sprintf("   ├─ Directories: %d items\n", len(dirs))
		summary += "   │  └─ Tip: Use recursive delete option\n"
	}

	// Unknown errors
	if unknown, ok := grouped[ErrorUnknown]; ok {
		summary += fmt.Sprintf("   └─ Other errors: %d files\n", len(unknown))
	}

	return summary
}
