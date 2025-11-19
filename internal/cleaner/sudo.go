package cleaner

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"
	"syscall"
	"time"

	"github.com/fenilsonani/cleanup-cache/internal/security"
	"golang.org/x/term"
)

// SudoManager handles sudo operations for file deletion
type SudoManager struct {
	password      []byte
	authenticated bool
	available     bool
	sessionExpiry time.Time
	pathValidator *security.PathValidator
}

// NewSudoManager creates a new sudo manager
func NewSudoManager() *SudoManager {
	return &SudoManager{
		available:     checkSudoAvailable(),
		pathValidator: security.NewPathValidator(),
	}
}

// checkSudoAvailable checks if sudo is available on the system
func checkSudoAvailable() bool {
	_, err := exec.LookPath("sudo")
	return err == nil
}

// IsAvailable returns whether sudo is available
func (sm *SudoManager) IsAvailable() bool {
	return sm.available
}

// IsAuthenticated returns whether we have a valid sudo session
func (sm *SudoManager) IsAuthenticated() bool {
	return sm.authenticated
}

// PromptForPassword prompts the user for their sudo password
func (sm *SudoManager) PromptForPassword() error {
	if !sm.available {
		return fmt.Errorf("sudo is not available on this system")
	}

	// Check if we already have a valid sudo session
	if sm.CheckSession() {
		sm.authenticated = true
		return nil
	}

	fmt.Print("🔐 Some files require elevated permissions.\n")
	fmt.Print("Please enter your password: ")

	// Read password without echoing
	passwordBytes, err := term.ReadPassword(int(syscall.Stdin))
	fmt.Println() // Print newline after password input

	if err != nil {
		return fmt.Errorf("failed to read password: %w", err)
	}

	if len(passwordBytes) == 0 {
		return fmt.Errorf("password cannot be empty")
	}

	// Validate password by running a simple sudo command
	if err := sm.validatePassword(passwordBytes); err != nil {
		return fmt.Errorf("authentication failed: %w", err)
	}

	sm.password = passwordBytes
	sm.authenticated = true
	sm.sessionExpiry = time.Now().Add(5 * time.Minute) // Sudo default timeout

	return nil
}

// validatePassword validates the sudo password by running a test command
func (sm *SudoManager) validatePassword(password []byte) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "sudo", "-S", "-v")

	// Create password input with newline
	passwordInput := append(password, '\n')
	cmd.Stdin = bytes.NewReader(passwordInput)

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		// Check if it's a wrong password error
		if strings.Contains(stderr.String(), "Sorry") {
			return fmt.Errorf("incorrect password")
		}
		if ctx.Err() == context.DeadlineExceeded {
			return fmt.Errorf("sudo command timed out")
		}
		return err
	}

	return nil
}

// CheckSession checks if there's an active sudo session
func (sm *SudoManager) CheckSession() bool {
	cmd := exec.Command("sudo", "-n", "true")
	return cmd.Run() == nil
}

// ensureAuthenticated checks if the session is still valid and refreshes if needed
func (sm *SudoManager) ensureAuthenticated() error {
	if !sm.authenticated {
		return fmt.Errorf("not authenticated: call PromptForPassword first")
	}

	// Check if session is about to expire (within 1 minute)
	if time.Now().After(sm.sessionExpiry.Add(-1 * time.Minute)) {
		if err := sm.KeepAlive(); err != nil {
			return fmt.Errorf("session expired and refresh failed: %w", err)
		}
	}

	return nil
}

// DeleteFile deletes a single file with sudo
func (sm *SudoManager) DeleteFile(path string) error {
	// Ensure we're still authenticated
	if err := sm.ensureAuthenticated(); err != nil {
		return err
	}

	// Validate path (includes symlink resolution to prevent traversal attacks)
	if err := sm.pathValidator.ValidatePathForDeletion(path); err != nil {
		return err
	}

	// Execute sudo rm with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "sudo", "-S", "rm", "-f", path)

	// Create password input with newline
	passwordInput := append(sm.password, '\n')
	cmd.Stdin = bytes.NewReader(passwordInput)

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return fmt.Errorf("sudo rm timed out for %s", path)
		}
		return fmt.Errorf("sudo rm failed: %w (%s)", err, stderr.String())
	}

	return nil
}

// DeleteFiles deletes multiple files with sudo in optimized batches
// Batches files into groups of 100 to reduce sudo process overhead
func (sm *SudoManager) DeleteFiles(paths []string) (succeeded []string, failed map[string]error) {
	succeeded = []string{}
	failed = make(map[string]error)

	if !sm.authenticated {
		for _, path := range paths {
			failed[path] = fmt.Errorf("not authenticated")
		}
		return
	}

	const batchSize = 100

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
	}

	return
}

// deleteBatch deletes a batch of files in a single sudo command
func (sm *SudoManager) deleteBatch(paths []string) (succeeded []string, failed map[string]error) {
	succeeded = []string{}
	failed = make(map[string]error)

	// Ensure we're still authenticated
	if err := sm.ensureAuthenticated(); err != nil {
		for _, path := range paths {
			failed[path] = err
		}
		return
	}

	// Validate all paths first
	validPaths := []string{}
	for _, path := range paths {
		if err := sm.pathValidator.ValidatePathForDeletion(path); err != nil {
			failed[path] = err
		} else {
			validPaths = append(validPaths, path)
		}
	}

	if len(validPaths) == 0 {
		return
	}

	// Build sudo rm command with all valid paths
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	args := []string{"-S", "rm", "-f"}
	args = append(args, validPaths...)

	cmd := exec.CommandContext(ctx, "sudo", args...)

	// Create password input with newline
	passwordInput := append(sm.password, '\n')
	cmd.Stdin = bytes.NewReader(passwordInput)

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			// Timeout - mark all as failed
			for _, path := range validPaths {
				failed[path] = fmt.Errorf("sudo rm batch timed out")
			}
			return
		}

		// Batch failed - fall back to individual deletion for this batch
		for _, path := range validPaths {
			if err := sm.DeleteFile(path); err != nil {
				failed[path] = err
			} else {
				succeeded = append(succeeded, path)
			}
		}
		return
	}

	// Batch succeeded - all files deleted
	succeeded = append(succeeded, validPaths...)
	return
}

// DeleteDirectory deletes a directory with sudo
func (sm *SudoManager) DeleteDirectory(path string, recursive bool) error {
	// Ensure we're still authenticated
	if err := sm.ensureAuthenticated(); err != nil {
		return err
	}

	// Validate path (includes symlink resolution to prevent traversal attacks)
	if err := sm.pathValidator.ValidatePathForDeletion(path); err != nil {
		return err
	}

	args := []string{"-S", "rm"}
	if recursive {
		args = append(args, "-rf")
	} else {
		args = append(args, "-d") // Only remove empty directory
	}
	args = append(args, path)

	// Execute with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "sudo", args...)

	// Create password input with newline
	passwordInput := append(sm.password, '\n')
	cmd.Stdin = bytes.NewReader(passwordInput)

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return fmt.Errorf("sudo rm timed out for %s", path)
		}
		return fmt.Errorf("sudo rm failed: %w (%s)", err, stderr.String())
	}

	return nil
}

// KeepAlive extends the sudo session timeout
func (sm *SudoManager) KeepAlive() error {
	if !sm.authenticated {
		return fmt.Errorf("not authenticated")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "sudo", "-S", "-v")

	// Create password input with newline
	passwordInput := append(sm.password, '\n')
	cmd.Stdin = bytes.NewReader(passwordInput)

	if err := cmd.Run(); err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			sm.authenticated = false
			return fmt.Errorf("keep-alive timed out")
		}
		sm.authenticated = false
		return fmt.Errorf("failed to keep sudo session alive: %w", err)
	}

	// Extend session expiry
	sm.sessionExpiry = time.Now().Add(5 * time.Minute)

	return nil
}

// Clear clears the password from memory and invalidates the session
func (sm *SudoManager) Clear() {
	// Securely overwrite password in memory
	if sm.password != nil {
		for i := range sm.password {
			sm.password[i] = 0
		}
		sm.password = nil
	}
	sm.authenticated = false
	sm.sessionExpiry = time.Time{}

	// Try to invalidate sudo session
	exec.Command("sudo", "-k").Run()
}
