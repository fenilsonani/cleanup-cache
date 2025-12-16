package reporter

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/fenilsonani/system-cleanup/internal/scanner"
	"github.com/fenilsonani/system-cleanup/pkg/utils"
	"gopkg.in/yaml.v3"
)

// OutputFormat represents the output format type
type OutputFormat string

const (
	FormatTable   OutputFormat = "table"
	FormatJSON    OutputFormat = "json"
	FormatYAML    OutputFormat = "yaml"
	FormatSummary OutputFormat = "summary"
)

// Reporter handles report generation
type Reporter struct {
	writer io.Writer
	format OutputFormat
}

// New creates a new Reporter
func New(writer io.Writer, format OutputFormat) *Reporter {
	return &Reporter{
		writer: writer,
		format: format,
	}
}

// Report generates a report from scan results
func (r *Reporter) Report(result *scanner.ScanResult) error {
	switch r.format {
	case FormatTable:
		return r.reportTable(result)
	case FormatJSON:
		return r.reportJSON(result)
	case FormatYAML:
		return r.reportYAML(result)
	case FormatSummary:
		return r.reportSummary(result)
	default:
		return fmt.Errorf("unsupported format: %s", r.format)
	}
}

// reportSummary generates a summary report
func (r *Reporter) reportSummary(result *scanner.ScanResult) error {
	fmt.Fprintf(r.writer, "=== Cleanup Summary ===\n")
	fmt.Fprintf(r.writer, "Total Files: %d\n", result.TotalCount)
	fmt.Fprintf(r.writer, "Total Size: %s\n", utils.FormatBytes(result.TotalSize))
	fmt.Fprintf(r.writer, "\nBreakdown by Category:\n")

	grouped := result.GroupByCategory()
	for category, catResult := range grouped {
		fmt.Fprintf(r.writer, "  %s: %d files, %s\n",
			category, catResult.TotalCount, utils.FormatBytes(catResult.TotalSize))
	}

	if len(result.Errors) > 0 {
		fmt.Fprintf(r.writer, "\nErrors: %d\n", len(result.Errors))
	}

	return nil
}

// reportTable generates a table report
func (r *Reporter) reportTable(result *scanner.ScanResult) error {
	// Print header
	fmt.Fprintf(r.writer, "%-60s | %-12s | %-20s | %s\n", "Path", "Size", "Category", "Modified")
	fmt.Fprintf(r.writer, "%s\n", string(make([]byte, 120)))

	// Print rows
	for _, file := range result.Files {
		path := file.Path
		if len(path) > 60 {
			path = "..." + path[len(path)-57:]
		}

		fmt.Fprintf(r.writer, "%-60s | %-12s | %-20s | %s\n",
			path,
			utils.FormatBytes(file.Size),
			file.Category,
			file.ModTime.Format("2006-01-02 15:04:05"))
	}

	// Print summary
	fmt.Fprintf(r.writer, "\n%s\n", string(make([]byte, 120)))
	fmt.Fprintf(r.writer, "Total: %d files, %s\n", result.TotalCount, utils.FormatBytes(result.TotalSize))

	return nil
}

// reportJSON generates a JSON report
func (r *Reporter) reportJSON(result *scanner.ScanResult) error {
	report := struct {
		Timestamp          string             `json:"timestamp"`
		TotalFiles         int                `json:"total_files"`
		TotalSize          int64              `json:"total_size"`
		TotalSizeFormatted string             `json:"total_size_formatted"`
		Files              []scanner.FileInfo `json:"files"`
		Errors             int                `json:"errors"`
	}{
		Timestamp:          time.Now().Format(time.RFC3339),
		TotalFiles:         result.TotalCount,
		TotalSize:          result.TotalSize,
		TotalSizeFormatted: utils.FormatBytes(result.TotalSize),
		Files:              result.Files,
		Errors:             len(result.Errors),
	}

	encoder := json.NewEncoder(r.writer)
	encoder.SetIndent("", "  ")
	return encoder.Encode(report)
}

// reportYAML generates a YAML report
func (r *Reporter) reportYAML(result *scanner.ScanResult) error {
	report := struct {
		Timestamp          string             `yaml:"timestamp"`
		TotalFiles         int                `yaml:"total_files"`
		TotalSize          int64              `yaml:"total_size"`
		TotalSizeFormatted string             `yaml:"total_size_formatted"`
		Files              []scanner.FileInfo `yaml:"files"`
		Errors             int                `yaml:"errors"`
	}{
		Timestamp:          time.Now().Format(time.RFC3339),
		TotalFiles:         result.TotalCount,
		TotalSize:          result.TotalSize,
		TotalSizeFormatted: utils.FormatBytes(result.TotalSize),
		Files:              result.Files,
		Errors:             len(result.Errors),
	}

	encoder := yaml.NewEncoder(r.writer)
	defer encoder.Close()
	return encoder.Encode(report)
}

// SaveToFile saves the report to a file
func SaveToFile(result *scanner.ScanResult, path string, format OutputFormat) error {
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()

	reporter := New(file, format)
	return reporter.Report(result)
}
