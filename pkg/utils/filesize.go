package utils

import "fmt"

const (
	B  = 1
	KB = 1024 * B
	MB = 1024 * KB
	GB = 1024 * MB
	TB = 1024 * GB
)

// FormatBytes converts bytes to human-readable format
func FormatBytes(bytes int64) string {
	if bytes < 0 {
		return "0 B"
	}

	switch {
	case bytes >= TB:
		return fmt.Sprintf("%.2f TB", float64(bytes)/float64(TB))
	case bytes >= GB:
		return fmt.Sprintf("%.2f GB", float64(bytes)/float64(GB))
	case bytes >= MB:
		return fmt.Sprintf("%.2f MB", float64(bytes)/float64(MB))
	case bytes >= KB:
		return fmt.Sprintf("%.2f KB", float64(bytes)/float64(KB))
	default:
		return fmt.Sprintf("%d B", bytes)
	}
}

// ParseSize converts human-readable size to bytes
func ParseSize(size string) (int64, error) {
	var value float64
	var unit string

	_, err := fmt.Sscanf(size, "%f%s", &value, &unit)
	if err != nil {
		return 0, fmt.Errorf("invalid size format: %s", size)
	}

	switch unit {
	case "B", "b":
		return int64(value), nil
	case "KB", "kb", "K", "k":
		return int64(value * KB), nil
	case "MB", "mb", "M", "m":
		return int64(value * MB), nil
	case "GB", "gb", "G", "g":
		return int64(value * GB), nil
	case "TB", "tb", "T", "t":
		return int64(value * TB), nil
	default:
		return 0, fmt.Errorf("unknown unit: %s", unit)
	}
}

// SumSizes adds up a slice of sizes
func SumSizes(sizes []int64) int64 {
	var total int64
	for _, size := range sizes {
		total += size
	}
	return total
}
