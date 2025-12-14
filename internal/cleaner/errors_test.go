package cleaner

import (
	"errors"
	"fmt"
	"os"
	"syscall"
	"testing"
)

func TestCategorizeError(t *testing.T) {
	tests := []struct {
		name      string
		err       error
		path      string
		reason    ErrorReason
		retryable bool
		needsSudo bool
	}{
		{
			name:      "EACCES - permission denied",
			err:       syscall.EACCES,
			path:      "/protected/file.txt",
			reason:    ErrorPermissionDenied,
			retryable: false,
			needsSudo: true,
		},
		{
			name:      "EPERM - operation not permitted",
			err:       syscall.EPERM,
			path:      "/system/file.txt",
			reason:    ErrorPermissionDenied,
			retryable: false,
			needsSudo: true,
		},
		{
			name:      "ENOENT - file not found",
			err:       syscall.ENOENT,
			path:      "/missing/file.txt",
			reason:    ErrorFileNotFound,
			retryable: false,
			needsSudo: false,
		},
		{
			name:      "EBUSY - resource busy",
			err:       syscall.EBUSY,
			path:      "/open/file.txt",
			reason:    ErrorFileInUse,
			retryable: true,
			needsSudo: false,
		},
		{
			name:      "EISDIR - is directory",
			err:       syscall.EISDIR,
			path:      "/some/dir",
			reason:    ErrorIsDirectory,
			retryable: false,
			needsSudo: false,
		},
		{
			name:      "wrapped EACCES",
			err:       fmt.Errorf("failed to remove: %w", syscall.EACCES),
			path:      "/wrapped/file.txt",
			reason:    ErrorPermissionDenied,
			retryable: false,
			needsSudo: true,
		},
		{
			name:      "os.PathError with EACCES",
			err:       &os.PathError{Op: "remove", Path: "/test/file.txt", Err: syscall.EACCES},
			path:      "/test/file.txt",
			reason:    ErrorPermissionDenied,
			retryable: false,
			needsSudo: true,
		},
		{
			name:      "os.IsNotExist error",
			err:       os.ErrNotExist,
			path:      "/not/exist.txt",
			reason:    ErrorFileNotFound,
			retryable: false,
			needsSudo: false,
		},
		{
			name:      "os.IsPermission error",
			err:       os.ErrPermission,
			path:      "/perm/denied.txt",
			reason:    ErrorPermissionDenied,
			retryable: false,
			needsSudo: true,
		},
		{
			name:      "generic error",
			err:       errors.New("unknown error"),
			path:      "/some/file.txt",
			reason:    ErrorUnknown,
			retryable: false,
			needsSudo: false,
		},
		{
			name:      "nil error",
			err:       nil,
			path:      "/nil/error/file.txt",
			reason:    ErrorUnknown,
			retryable: false,
			needsSudo: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			delErr := CategorizeError(tt.path, tt.err)

			if tt.err == nil {
				if delErr != nil {
					t.Errorf("CategorizeError(nil) should return nil, got %v", delErr)
				}
				return
			}

			if delErr.Reason != tt.reason {
				t.Errorf("CategorizeError(%v) reason = %v, want %v", tt.err, delErr.Reason, tt.reason)
			}

			if delErr.Retryable != tt.retryable {
				t.Errorf("CategorizeError(%v) retryable = %v, want %v", tt.err, delErr.Retryable, tt.retryable)
			}

			if delErr.NeedsSudo != tt.needsSudo {
				t.Errorf("CategorizeError(%v) needsSudo = %v, want %v", tt.err, delErr.NeedsSudo, tt.needsSudo)
			}

			if delErr.Path != tt.path {
				t.Errorf("CategorizeError(%v) path = %s, want %s", tt.err, delErr.Path, tt.path)
			}
		})
	}
}

func TestDeletionError_Error(t *testing.T) {
	tests := []struct {
		name          string
		delErr        *DeletionError
		shouldContain string
	}{
		{
			name: "permission error",
			delErr: &DeletionError{
				Path:      "/test/file.txt",
				Reason:    ErrorPermissionDenied,
				Original:  os.ErrPermission,
				Retryable: false,
			},
			shouldContain: "/test/file.txt",
		},
		{
			name: "busy error (retryable)",
			delErr: &DeletionError{
				Path:      "/test/open.txt",
				Reason:    ErrorFileInUse,
				Original:  errors.New("resource busy"),
				Retryable: true,
			},
			shouldContain: "/test/open.txt",
		},
		{
			name: "not found error",
			delErr: &DeletionError{
				Path:      "/missing/file.txt",
				Reason:    ErrorFileNotFound,
				Original:  os.ErrNotExist,
				Retryable: false,
			},
			shouldContain: "/missing/file.txt",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.delErr.Error()
			if !stringContains(result, tt.shouldContain) {
				t.Errorf("DeletionError.Error() = %s, should contain %s", result, tt.shouldContain)
			}
		})
	}
}

func TestFormatErrorSummary(t *testing.T) {
	delErrors := []*DeletionError{
		{
			Path:      "/test/file1.txt",
			Reason:    ErrorPermissionDenied,
			Original:  os.ErrPermission,
			Retryable: false,
		},
		{
			Path:      "/test/file2.txt",
			Reason:    ErrorPermissionDenied,
			Original:  os.ErrPermission,
			Retryable: false,
		},
		{
			Path:      "/test/file3.txt",
			Reason:    ErrorFileInUse,
			Original:  errors.New("busy"),
			Retryable: true,
		},
		{
			Path:      "/test/file4.txt",
			Reason:    ErrorFileNotFound,
			Original:  os.ErrNotExist,
			Retryable: false,
		},
		{
			Path:      "/test/file5.txt",
			Reason:    ErrorUnknown,
			Original:  errors.New("unknown"),
			Retryable: false,
		},
	}

	summary := FormatErrorSummary(delErrors)

	// Verify summary contains error information
	if !stringContains(summary, "Permission denied") {
		t.Error("Expected summary to contain 'Permission denied'")
	}

	if !stringContains(summary, "2") { // 2 permission errors
		t.Error("Expected summary to show count of 2 for permission errors")
	}

	if !stringContains(summary, "File in use") {
		t.Error("Expected summary to contain 'File in use'")
	}

	// Test with empty errors
	emptySummary := FormatErrorSummary([]*DeletionError{})
	if emptySummary != "" {
		t.Errorf("Expected empty summary for no errors, got: %s", emptySummary)
	}

	// Test with nil
	nilSummary := FormatErrorSummary(nil)
	if nilSummary != "" {
		t.Errorf("Expected empty summary for nil errors, got: %s", nilSummary)
	}
}

func TestErrorReasonString(t *testing.T) {
	tests := []struct {
		reason   ErrorReason
		expected string
	}{
		{ErrorPermissionDenied, "Permission denied"},
		{ErrorFileNotFound, "File not found"},
		{ErrorFileInUse, "File is in use"},
		{ErrorIsDirectory, "Is a directory"},
		{ErrorInvalidPath, "Invalid path"},
		{ErrorUnknown, "Unknown error"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			result := tt.reason.String()
			if result != tt.expected {
				t.Errorf("ErrorReason(%d).String() = %s, want %s", tt.reason, result, tt.expected)
			}
		})
	}
}

func TestGroupErrors(t *testing.T) {
	delErrors := []*DeletionError{
		{Reason: ErrorPermissionDenied, Path: "/a", Original: os.ErrPermission},
		{Reason: ErrorPermissionDenied, Path: "/b", Original: os.ErrPermission},
		{Reason: ErrorFileInUse, Path: "/c", Original: errors.New("busy")},
		{Reason: ErrorFileNotFound, Path: "/d", Original: os.ErrNotExist},
		{Reason: ErrorFileInUse, Path: "/e", Original: errors.New("busy")},
	}

	grouped := GroupErrors(delErrors)

	// Verify grouping
	if len(grouped[ErrorPermissionDenied]) != 2 {
		t.Errorf("Expected 2 permission errors, got %d", len(grouped[ErrorPermissionDenied]))
	}

	if len(grouped[ErrorFileInUse]) != 2 {
		t.Errorf("Expected 2 busy errors, got %d", len(grouped[ErrorFileInUse]))
	}

	if len(grouped[ErrorFileNotFound]) != 1 {
		t.Errorf("Expected 1 not found error, got %d", len(grouped[ErrorFileNotFound]))
	}

	if len(grouped[ErrorUnknown]) != 0 {
		t.Errorf("Expected 0 unknown errors, got %d", len(grouped[ErrorUnknown]))
	}
}

func TestUnwrapError(t *testing.T) {
	// Test unwrapping to find root cause
	rootErr := syscall.EACCES
	wrappedOnce := fmt.Errorf("level 1: %w", rootErr)
	wrappedTwice := fmt.Errorf("level 2: %w", wrappedOnce)

	delErr := CategorizeError("/test/file.txt", wrappedTwice)

	if delErr.Reason != ErrorPermissionDenied {
		t.Error("Expected error unwrapping to find EACCES and categorize as permission error")
	}

	// Test with os.PathError wrapping
	pathErr := &os.PathError{
		Op:   "remove",
		Path: "/test/wrapped.txt",
		Err:  syscall.EBUSY,
	}
	wrappedPathErr := fmt.Errorf("failed: %w", pathErr)

	delErr = CategorizeError("/test/wrapped.txt", wrappedPathErr)

	if delErr.Reason != ErrorFileInUse {
		t.Error("Expected error unwrapping through PathError to find EBUSY")
	}

	if !delErr.Retryable {
		t.Error("Expected EBUSY error to be retryable")
	}
}

func TestDeletionError_UserMessage(t *testing.T) {
	tests := []struct {
		name          string
		delErr        *DeletionError
		shouldContain string
	}{
		{
			name: "permission denied with sudo",
			delErr: &DeletionError{
				Path:      "/test/file.txt",
				Reason:    ErrorPermissionDenied,
				NeedsSudo: true,
				Original:  os.ErrPermission,
			},
			shouldContain: "elevated permissions",
		},
		{
			name: "file in use",
			delErr: &DeletionError{
				Path:     "/test/open.txt",
				Reason:   ErrorFileInUse,
				Original: errors.New("busy"),
			},
			shouldContain: "being used",
		},
		{
			name: "file not found",
			delErr: &DeletionError{
				Path:     "/test/missing.txt",
				Reason:   ErrorFileNotFound,
				Original: os.ErrNotExist,
			},
			shouldContain: "Already deleted",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.delErr.UserMessage()
			if !stringContains(result, tt.shouldContain) {
				t.Errorf("UserMessage() = %s, should contain %s", result, tt.shouldContain)
			}
		})
	}
}
