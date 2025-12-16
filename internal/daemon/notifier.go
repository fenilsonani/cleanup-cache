package daemon

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/smtp"
	"text/template"
	"time"

	"github.com/fenilsonani/system-cleanup/internal/cleaner"
	"github.com/fenilsonani/system-cleanup/internal/config"
)

// Notifier handles notifications for the daemon
type Notifier struct {
	config *config.NotificationConfig
	logger *Logger
}

// NewNotifier creates a new notifier
func NewNotifier(cfg *config.NotificationConfig, logger *Logger) *Notifier {
	return &Notifier{
		config: cfg,
		logger: logger,
	}
}

// NotificationMessage represents a notification
type NotificationMessage struct {
	Title     string
	Message   string
	Timestamp time.Time
	Type      string // "startup", "shutdown", "cleanup_success", "cleanup_failure"
	Data      map[string]interface{}
}

// SendStartupNotification sends a startup notification
func (n *Notifier) SendStartupNotification() {
	if !n.config.Enabled {
		return
	}

	msg := &NotificationMessage{
		Title:     "CleanupCache Daemon Started",
		Message:   "The cleanup daemon has started successfully",
		Timestamp: time.Now(),
		Type:      "startup",
	}

	n.sendAll(msg)
}

// SendShutdownNotification sends a shutdown notification
func (n *Notifier) SendShutdownNotification() {
	if !n.config.Enabled {
		return
	}

	msg := &NotificationMessage{
		Title:     "CleanupCache Daemon Stopped",
		Message:   "The cleanup daemon has stopped",
		Timestamp: time.Now(),
		Type:      "shutdown",
	}

	n.sendAll(msg)
}

// SendCleanupNotification sends a cleanup notification
func (n *Notifier) SendCleanupNotification(job *CleanupJob, result *cleaner.CleanResult, duration time.Duration) {
	if !n.config.Enabled {
		return
	}

	// Determine if this is a success or failure
	hasErrors := len(result.Errors) > 0
	notificationType := "cleanup_success"
	if hasErrors {
		notificationType = "cleanup_failure"
	}

	// Only send based on configuration
	if hasErrors && !n.config.OnFailure {
		return
	}
	if !hasErrors && !n.config.OnSuccess {
		return
	}

	msg := &NotificationMessage{
		Timestamp: time.Now(),
		Type:      notificationType,
		Data: map[string]interface{}{
			"job_name":      job.Name,
			"files_deleted": len(result.DeletedFiles),
			"space_freed":   result.DeletedSize,
			"errors":        len(result.Errors),
			"duration":      duration.String(),
			"dry_run":       result.DryRun,
		},
	}

	if hasErrors {
		msg.Title = fmt.Sprintf("Cleanup Failed: %s", job.Name)
		msg.Message = fmt.Sprintf("Cleanup job completed with %d errors. Deleted %d files, freed %s",
			len(result.Errors), len(result.DeletedFiles), formatBytes(result.DeletedSize))
	} else {
		msg.Title = fmt.Sprintf("Cleanup Completed: %s", job.Name)
		msg.Message = fmt.Sprintf("Successfully deleted %d files, freed %s in %s",
			len(result.DeletedFiles), formatBytes(result.DeletedSize), duration.Round(time.Second))
	}

	n.sendAll(msg)
}

// sendAll sends notification through all configured channels
func (n *Notifier) sendAll(msg *NotificationMessage) {
	// Send email
	if n.config.Email.SMTPHost != "" {
		if err := n.sendEmail(msg); err != nil {
			n.logger.Error("Failed to send email notification: %v", err)
		} else {
			n.logger.Info("Email notification sent: %s", msg.Title)
		}
	}

	// Send webhook
	if n.config.Webhook.URL != "" {
		if err := n.sendWebhook(msg); err != nil {
			n.logger.Error("Failed to send webhook notification: %v", err)
		} else {
			n.logger.Info("Webhook notification sent: %s", msg.Title)
		}
	}
}

// sendEmail sends an email notification
func (n *Notifier) sendEmail(msg *NotificationMessage) error {
	cfg := &n.config.Email

	if len(cfg.To) == 0 {
		return fmt.Errorf("no email recipients configured")
	}

	// Build email body
	body, err := n.buildEmailBody(msg)
	if err != nil {
		return fmt.Errorf("failed to build email body: %w", err)
	}

	// Build message
	emailMsg := fmt.Sprintf("To: %s\r\nSubject: %s\r\nContent-Type: text/html; charset=UTF-8\r\n\r\n%s",
		cfg.To[0], msg.Title, body)

	// Connect and send
	auth := smtp.PlainAuth("", cfg.Username, cfg.Password, cfg.SMTPHost)
	addr := fmt.Sprintf("%s:%d", cfg.SMTPHost, cfg.SMTPPort)

	return smtp.SendMail(addr, auth, cfg.From, cfg.To, []byte(emailMsg))
}

// buildEmailBody builds the HTML email body
func (n *Notifier) buildEmailBody(msg *NotificationMessage) (string, error) {
	tmpl := `<!DOCTYPE html>
<html>
<head>
    <style>
        body { font-family: Arial, sans-serif; margin: 20px; }
        .header { background-color: #2c3e50; color: white; padding: 20px; border-radius: 5px 5px 0 0; }
        .content { padding: 20px; border: 1px solid #ddd; border-top: none; }
        .footer { font-size: 12px; color: #666; margin-top: 20px; }
        .success { color: #27ae60; }
        .failure { color: #e74c3c; }
        .info { color: #3498db; }
        table { border-collapse: collapse; width: 100%; margin-top: 15px; }
        th, td { padding: 8px; text-align: left; border-bottom: 1px solid #ddd; }
        th { background-color: #f5f5f5; }
    </style>
</head>
<body>
    <div class="header">
        <h2>{{.Title}}</h2>
    </div>
    <div class="content">
        <p>{{.Message}}</p>
        <p><strong>Time:</strong> {{.Timestamp.Format "2006-01-02 15:04:05"}}</p>
        {{if .Data}}
        <table>
            <tr><th>Metric</th><th>Value</th></tr>
            {{range $key, $value := .Data}}
            <tr><td>{{$key}}</td><td>{{$value}}</td></tr>
            {{end}}
        </table>
        {{end}}
    </div>
    <div class="footer">
        <p>Sent by CleanupCache Daemon</p>
    </div>
</body>
</html>`

	t, err := template.New("email").Parse(tmpl)
	if err != nil {
		return "", err
	}

	var buf bytes.Buffer
	if err := t.Execute(&buf, msg); err != nil {
		return "", err
	}

	return buf.String(), nil
}

// sendWebhook sends a webhook notification
func (n *Notifier) sendWebhook(msg *NotificationMessage) error {
	cfg := &n.config.Webhook

	// Build payload
	payload := map[string]interface{}{
		"title":     msg.Title,
		"message":   msg.Message,
		"timestamp": msg.Timestamp.Format(time.RFC3339),
		"type":      msg.Type,
		"data":      msg.Data,
	}

	// Convert to JSON
	jsonData, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal payload: %w", err)
	}

	// Create request
	method := cfg.Method
	if method == "" {
		method = "POST"
	}

	req, err := http.NewRequest(method, cfg.URL, bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers
	req.Header.Set("Content-Type", "application/json")
	for key, value := range cfg.Headers {
		req.Header.Set(key, value)
	}

	// Send request
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send webhook: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("webhook returned status %d", resp.StatusCode)
	}

	return nil
}

// formatBytes formats bytes to human-readable string
func formatBytes(bytes int64) string {
	const (
		KB = 1024
		MB = 1024 * KB
		GB = 1024 * MB
	)

	switch {
	case bytes >= GB:
		return fmt.Sprintf("%.2f GB", float64(bytes)/GB)
	case bytes >= MB:
		return fmt.Sprintf("%.2f MB", float64(bytes)/MB)
	case bytes >= KB:
		return fmt.Sprintf("%.2f KB", float64(bytes)/KB)
	default:
		return fmt.Sprintf("%d bytes", bytes)
	}
}
