package cleaner

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/fenilsonani/system-cleanup/internal/security"
	"golang.org/x/term"
)

// SudoManager handles sudo operations for file deletion with enhanced reliability
type SudoManager struct {
	password        []byte
	authenticated   bool
	available       bool
	sessionExpiry   time.Time
	pathValidator   *security.PathValidator
	mu              sync.RWMutex
	failedPaths     map[string]int // Track retry counts per path
	maxRetries      int
	usePolkit       bool           // Whether to use pkexec as fallback
	polkitAvailable bool
}

// SudoConfig contains configuration for the sudo manager
type SudoConfig struct {
	MaxRetries      int
	SessionTimeout  time.Duration
	CommandTimeout  time.Duration
	BatchSize       int
	UsePolkitFallback bool
}

// DefaultSudoConfig returns sensible defaults
func DefaultSudoConfig() *SudoConfig {
	return &SudoConfig{
		MaxRetries:        3,
		SessionTimeout:    5 * time.Minute,
		CommandTimeout:    30 * time.Second,
		BatchSize:         50, // Reduced from 100 for better reliability
		UsePolkitFallback: true,
	}
}

// NewSudoManager creates a new sudo manager with enhanced capabilities
func NewSudoManager() *SudoManager {
	sm := &SudoManager{
		available:       checkSudoAvailable(),
		polkitAvailable: checkPolkitAvailable(),
		pathValidator:   security.NewPathValidator(),
		failedPaths:     make(map[string]int),
		maxRetries:      3,
	}
	return sm
}

// NewSudoManagerWithConfig creates a sudo manager with custom configuration
func NewSudoManagerWithConfig(cfg *SudoConfig) *SudoManager {
	sm := NewSudoManager()
	if cfg != nil {
		sm.maxRetries = cfg.MaxRetries
		sm.usePolkit = cfg.UsePolkitFallback
	}
	return sm
}

// checkSudoAvailable checks if sudo is available on the system
func checkSudoAvailable() bool {
	_, err := exec.LookPath("sudo")
	return err == nil
}

// checkPolkitAvailable checks if pkexec is available as a fallback
func checkPolkitAvailable() bool {
	_, err := exec.LookPath("pkexec")
	return err == nil
}

// IsAvailable returns whether sudo is available
func (sm *SudoManager) IsAvailable() bool {
	return sm.available
}

// IsPolkitAvailable returns whether pkexec is available as fallback
func (sm *SudoManager) IsPolkitAvailable() bool {
	return sm.polkitAvailable
}

// IsAuthenticated returns whether we have a valid sudo session
func (sm *SudoManager) IsAuthenticated() bool {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	return sm.authenticated && time.Now().Before(sm.sessionExpiry)
}

// PromptForPassword prompts the user for their sudo password with improved error handling
func (sm *SudoManager) PromptForPassword() error {
	if !sm.available {
		return fmt.Errorf("sudo is not available on this system")
	}

	// Check if we already have a valid sudo session (passwordless or cached)
	if sm.CheckSession() {
		sm.mu.Lock()
		sm.authenticated = true
		sm.sessionExpiry = time.Now().Add(5 * time.Minute)
		sm.mu.Unlock()
		return nil
	}

	fmt.Print("\n🔐 Some files require elevated permissions.\n")
	fmt.Print("Please enter your password (or press Ctrl+C to skip): ")

	// Read password without echoing
	passwordBytes, err := term.ReadPassword(int(syscall.Stdin))
	fmt.Println() // Print newline after password input

	if err != nil {
		return fmt.Errorf("failed to read password: %w", err)
	}

	if len(passwordBytes) == 0 {
		return fmt.Errorf("password cannot be empty")
	}

	// Validate password by running a simple sudo command with retries
	var lastErr error
	for attempt := 1; attempt <= 3; attempt++ {
		if err := sm.validatePassword(passwordBytes); err != nil {
			lastErr = err
			if strings.Contains(err.Error(), "incorrect password") {
				// Wrong password, clear and fail
				clearBytes(passwordBytes)
				return fmt.Errorf("authentication failed: incorrect password")
			}
			// Other error, retry
			time.Sleep(time.Duration(attempt*100) * time.Millisecond)
			continue
		}
		// Success
		sm.mu.Lock()
		sm.password = passwordBytes
		sm.authenticated = true
		sm.sessionExpiry = time.Now().Add(5 * time.Minute)
		sm.mu.Unlock()
		return nil
	}

	clearBytes(passwordBytes)
	return fmt.Errorf("authentication failed after 3 attempts: %w", lastErr)
}

// validatePassword validates the sudo password by running a test command
func (sm *SudoManager) validatePassword(password []byte) error {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "sudo", "-S", "-v")

	// Create password input with newline
	passwordInput := append([]byte(nil), password...)
	passwordInput = append(passwordInput, '\n')
	cmd.Stdin = bytes.NewReader(passwordInput)

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		stderrStr := stderr.String()
		// Check for common error messages
		if strings.Contains(stderrStr, "Sorry") || strings.Contains(stderrStr, "incorrect password") {
			return fmt.Errorf("incorrect password")
		}
		if strings.Contains(stderrStr, "try again") {
			return fmt.Errorf("incorrect password")
		}
		if ctx.Err() == context.DeadlineExceeded {
			return fmt.Errorf("sudo command timed out")
		}
		return fmt.Errorf("sudo validation failed: %v (stderr: %s)", err, stderrStr)
	}

	return nil
}

// CheckSession checks if there's an active sudo session (passwordless)
func (sm *SudoManager) CheckSession() bool {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "sudo", "-n", "true")
	return cmd.Run() == nil
}

// ensureAuthenticated checks if the session is still valid and refreshes if needed
func (sm *SudoManager) ensureAuthenticated() error {
	sm.mu.RLock()
	if !sm.authenticated {
		sm.mu.RUnlock()
		return fmt.Errorf("not authenticated: call PromptForPassword first")
	}

	// Check if session is about to expire (within 1 minute)
	needsRefresh := time.Now().After(sm.sessionExpiry.Add(-1 * time.Minute))
	sm.mu.RUnlock()

	if needsRefresh {
		if err := sm.KeepAlive(); err != nil {
			return fmt.Errorf("session expired and refresh failed: %w", err)
		}
	}

	return nil
}

// SudoDeletionResult contains the result of a sudo deletion attempt
type SudoDeletionResult struct {
	Path      string
	Success   bool
	Error     error
	Method    string // "sudo", "pkexec", "direct", "rename-delete"
	Attempts  int
	Duration  time.Duration
}

// DeleteFile deletes a single file with sudo using multiple strategies
func (sm *SudoManager) DeleteFile(path string) error {
	result := sm.DeleteFileWithResult(path)
	return result.Error
}

// DeleteFileWithResult deletes a file and returns detailed result
func (sm *SudoManager) DeleteFileWithResult(path string) *SudoDeletionResult {
	result := &SudoDeletionResult{
		Path:   path,
		Method: "sudo",
	}
	startTime := time.Now()
	defer func() {
		result.Duration = time.Since(startTime)
	}()

	// Validate path first
	if err := sm.pathValidator.ValidatePathForDeletion(path); err != nil {
		result.Error = fmt.Errorf("path validation failed: %w", err)
		return result
	}

	// Check if file still exists
	if _, err := os.Lstat(path); os.IsNotExist(err) {
		result.Success = true // Already deleted
		return result
	}

	// Try different deletion strategies with retries
	strategies := []func(string) error{
		sm.deleteWithSudo,
		sm.deleteWithRenameFirst,
	}

	// Add pkexec fallback if available
	if sm.polkitAvailable && sm.usePolkit {
		strategies = append(strategies, sm.deleteWithPkexec)
	}

	var lastErr error
	for _, strategy := range strategies {
		for attempt := 1; attempt <= sm.maxRetries; attempt++ {
			result.Attempts++

			err := strategy(path)
			if err == nil {
				// Verify deletion
				if _, verifyErr := os.Lstat(path); os.IsNotExist(verifyErr) {
					result.Success = true
					return result
				}
				// File still exists despite no error, continue trying
				lastErr = fmt.Errorf("deletion reported success but file still exists")
				continue
			}

			lastErr = err

			// Check if error is retryable
			if !isRetryableError(err) {
				break // Move to next strategy
			}

			// Exponential backoff
			backoff := time.Duration(attempt*attempt*100) * time.Millisecond
			time.Sleep(backoff)
		}
	}

	result.Error = lastErr
	return result
}

// deleteWithSudo deletes using standard sudo rm
func (sm *SudoManager) deleteWithSudo(path string) error {
	if err := sm.ensureAuthenticated(); err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "sudo", "-S", "rm", "-f", "--", path)

	sm.mu.RLock()
	passwordInput := append([]byte(nil), sm.password...)
	sm.mu.RUnlock()
	passwordInput = append(passwordInput, '\n')
	cmd.Stdin = bytes.NewReader(passwordInput)

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return fmt.Errorf("sudo rm timed out")
		}
		return fmt.Errorf("sudo rm failed: %w (stderr: %s)", err, strings.TrimSpace(stderr.String()))
	}

	return nil
}

// deleteWithRenameFirst renames the file before deletion (atomic delete strategy)
func (sm *SudoManager) deleteWithRenameFirst(path string) error {
	if err := sm.ensureAuthenticated(); err != nil {
		return err
	}

	// Generate a random temporary name
	randomBytes := make([]byte, 8)
	rand.Read(randomBytes)
	tempName := filepath.Join(filepath.Dir(path), ".deleted_"+hex.EncodeToString(randomBytes))

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// First rename the file
	sm.mu.RLock()
	passwordInput := append([]byte(nil), sm.password...)
	sm.mu.RUnlock()
	passwordInput = append(passwordInput, '\n')

	renameCmd := exec.CommandContext(ctx, "sudo", "-S", "mv", "-f", "--", path, tempName)
	renameCmd.Stdin = bytes.NewReader(passwordInput)

	var renameStderr bytes.Buffer
	renameCmd.Stderr = &renameStderr

	if err := renameCmd.Run(); err != nil {
		return fmt.Errorf("rename failed: %w (stderr: %s)", err, strings.TrimSpace(renameStderr.String()))
	}

	// Now delete the renamed file
	deleteCmd := exec.CommandContext(ctx, "sudo", "-S", "rm", "-f", "--", tempName)
	sm.mu.RLock()
	passwordInput2 := append([]byte(nil), sm.password...)
	sm.mu.RUnlock()
	passwordInput2 = append(passwordInput2, '\n')
	deleteCmd.Stdin = bytes.NewReader(passwordInput2)

	var deleteStderr bytes.Buffer
	deleteCmd.Stderr = &deleteStderr

	if err := deleteCmd.Run(); err != nil {
		// Try to restore original name on failure
		restoreCmd := exec.CommandContext(ctx, "sudo", "-S", "mv", "-f", "--", tempName, path)
		sm.mu.RLock()
		passwordInput3 := append([]byte(nil), sm.password...)
		sm.mu.RUnlock()
		passwordInput3 = append(passwordInput3, '\n')
		restoreCmd.Stdin = bytes.NewReader(passwordInput3)
		restoreCmd.Run() // Best effort restore

		return fmt.Errorf("delete after rename failed: %w", err)
	}

	return nil
}

// deleteWithPkexec uses pkexec as a fallback for graphical environments
func (sm *SudoManager) deleteWithPkexec(path string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "pkexec", "rm", "-f", "--", path)

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return fmt.Errorf("pkexec rm timed out")
		}
		return fmt.Errorf("pkexec rm failed: %w (stderr: %s)", err, strings.TrimSpace(stderr.String()))
	}

	return nil
}

// DeleteFiles deletes multiple files with sudo in optimized batches
func (sm *SudoManager) DeleteFiles(paths []string) (succeeded []string, failed map[string]error) {
	succeeded = make([]string, 0, len(paths))
	failed = make(map[string]error)

	sm.mu.RLock()
	if !sm.authenticated {
		sm.mu.RUnlock()
		for _, path := range paths {
			failed[path] = fmt.Errorf("not authenticated")
		}
		return
	}
	sm.mu.RUnlock()

	const batchSize = 50 // Smaller batches for better reliability

	// Process files in batches
	for i := 0; i < len(paths); i += batchSize {
		end := i + batchSize
		if end > len(paths) {
			end = len(paths)
		}

		batch := paths[i:end]
		batchSucceeded, batchFailed := sm.deleteBatch(batch)

		succeeded = append(succeeded, batchSucceeded...)
		for path, err := range batchFailed {
			failed[path] = err
		}

		// Keep session alive between batches
		if len(paths) > batchSize && i+batchSize < len(paths) {
			sm.KeepAlive()
		}
	}

	return
}

// deleteBatch deletes a batch of files with improved error handling
func (sm *SudoManager) deleteBatch(paths []string) (succeeded []string, failed map[string]error) {
	succeeded = make([]string, 0, len(paths))
	failed = make(map[string]error)

	if err := sm.ensureAuthenticated(); err != nil {
		for _, path := range paths {
			failed[path] = err
		}
		return
	}

	// Validate all paths first
	validPaths := make([]string, 0, len(paths))
	for _, path := range paths {
		if err := sm.pathValidator.ValidatePathForDeletion(path); err != nil {
			failed[path] = fmt.Errorf("validation failed: %w", err)
		} else {
			// Also check file still exists
			if _, err := os.Lstat(path); err != nil {
				if os.IsNotExist(err) {
					succeeded = append(succeeded, path) // Already deleted
				} else {
					failed[path] = err
				}
			} else {
				validPaths = append(validPaths, path)
			}
		}
	}

	if len(validPaths) == 0 {
		return
	}

	// Build sudo rm command with all valid paths
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	// Use xargs approach for very large batches to avoid arg limit
	args := []string{"-S", "rm", "-f", "--"}
	args = append(args, validPaths...)

	cmd := exec.CommandContext(ctx, "sudo", args...)

	sm.mu.RLock()
	passwordInput := append([]byte(nil), sm.password...)
	sm.mu.RUnlock()
	passwordInput = append(passwordInput, '\n')
	cmd.Stdin = bytes.NewReader(passwordInput)

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		stderrStr := strings.TrimSpace(stderr.String())

		if ctx.Err() == context.DeadlineExceeded {
			// Timeout - check which files were actually deleted
			for _, path := range validPaths {
				if _, statErr := os.Lstat(path); os.IsNotExist(statErr) {
					succeeded = append(succeeded, path)
				} else {
					failed[path] = fmt.Errorf("batch timed out: %s", stderrStr)
				}
			}
			return
		}

		// Batch failed - fall back to individual deletion
		for _, path := range validPaths {
			// Check if already deleted
			if _, statErr := os.Lstat(path); os.IsNotExist(statErr) {
				succeeded = append(succeeded, path)
				continue
			}

			// Try individual deletion
			result := sm.DeleteFileWithResult(path)
			if result.Success {
				succeeded = append(succeeded, path)
			} else {
				failed[path] = result.Error
			}
		}
		return
	}

	// Verify each file was actually deleted
	for _, path := range validPaths {
		if _, statErr := os.Lstat(path); os.IsNotExist(statErr) {
			succeeded = append(succeeded, path)
		} else {
			// File still exists, try individual deletion
			result := sm.DeleteFileWithResult(path)
			if result.Success {
				succeeded = append(succeeded, path)
			} else {
				failed[path] = fmt.Errorf("batch succeeded but file not deleted: %v", result.Error)
			}
		}
	}

	return
}

// DeleteDirectory deletes a directory with sudo
func (sm *SudoManager) DeleteDirectory(path string, recursive bool) error {
	if err := sm.ensureAuthenticated(); err != nil {
		return err
	}

	if err := sm.pathValidator.ValidatePathForDeletion(path); err != nil {
		return err
	}

	// Extra safety check for directories
	info, err := os.Lstat(path)
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return fmt.Errorf("path is not a directory: %s", path)
	}

	args := []string{"-S", "rm"}
	if recursive {
		args = append(args, "-rf", "--")
	} else {
		args = append(args, "-d", "--") // Only remove empty directory
	}
	args = append(args, path)

	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "sudo", args...)

	sm.mu.RLock()
	passwordInput := append([]byte(nil), sm.password...)
	sm.mu.RUnlock()
	passwordInput = append(passwordInput, '\n')
	cmd.Stdin = bytes.NewReader(passwordInput)

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return fmt.Errorf("sudo rm directory timed out for %s", path)
		}
		return fmt.Errorf("sudo rm directory failed: %w (stderr: %s)", err, strings.TrimSpace(stderr.String()))
	}

	// Verify deletion
	if _, err := os.Lstat(path); !os.IsNotExist(err) {
		return fmt.Errorf("directory still exists after deletion")
	}

	return nil
}

// KeepAlive extends the sudo session timeout
func (sm *SudoManager) KeepAlive() error {
	sm.mu.RLock()
	if !sm.authenticated {
		sm.mu.RUnlock()
		return fmt.Errorf("not authenticated")
	}
	password := append([]byte(nil), sm.password...)
	sm.mu.RUnlock()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "sudo", "-S", "-v")

	passwordInput := append(password, '\n')
	cmd.Stdin = bytes.NewReader(passwordInput)

	if err := cmd.Run(); err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			sm.mu.Lock()
			sm.authenticated = false
			sm.mu.Unlock()
			return fmt.Errorf("keep-alive timed out")
		}
		sm.mu.Lock()
		sm.authenticated = false
		sm.mu.Unlock()
		return fmt.Errorf("failed to keep sudo session alive: %w", err)
	}

	sm.mu.Lock()
	sm.sessionExpiry = time.Now().Add(5 * time.Minute)
	sm.mu.Unlock()

	return nil
}

// Clear clears the password from memory and invalidates the session
func (sm *SudoManager) Clear() {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	// Securely overwrite password in memory
	if sm.password != nil {
		clearBytes(sm.password)
		sm.password = nil
	}
	sm.authenticated = false
	sm.sessionExpiry = time.Time{}

	// Clear failed paths tracking
	sm.failedPaths = make(map[string]int)

	// Try to invalidate sudo session (non-blocking)
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		exec.CommandContext(ctx, "sudo", "-k").Run()
	}()
}

// GetStatistics returns statistics about sudo operations
func (sm *SudoManager) GetStatistics() map[string]interface{} {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	return map[string]interface{}{
		"authenticated":      sm.authenticated,
		"available":          sm.available,
		"polkit_available":   sm.polkitAvailable,
		"session_expiry":     sm.sessionExpiry,
		"failed_paths_count": len(sm.failedPaths),
	}
}

// isRetryableError determines if an error should trigger a retry
func isRetryableError(err error) bool {
	if err == nil {
		return false
	}

	errStr := strings.ToLower(err.Error())

	// Retryable conditions
	retryable := []string{
		"resource temporarily unavailable",
		"text file busy",
		"device or resource busy",
		"operation timed out",
		"connection refused",
		"broken pipe",
		"no such process",
	}

	for _, r := range retryable {
		if strings.Contains(errStr, r) {
			return true
		}
	}

	// Not retryable conditions
	notRetryable := []string{
		"permission denied",
		"operation not permitted",
		"no such file or directory",
		"read-only file system",
		"incorrect password",
		"authentication failed",
	}

	for _, nr := range notRetryable {
		if strings.Contains(errStr, nr) {
			return false
		}
	}

	return true // Default to retrying for unknown errors
}

// clearBytes securely zeros a byte slice
func clearBytes(b []byte) {
	for i := range b {
		b[i] = 0
	}
}
