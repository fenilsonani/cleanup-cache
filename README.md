# TidyUp 🗑️

A powerful, safe, and intelligent CLI tool to clean up your Mac or Linux system by removing unnecessary files, caches, logs, and more.

## ✨ Features

- 🔍 **Smart Scanning** - Intelligently identifies cleanable files across multiple categories
- 🔒 **Safe & Secure** - Multiple safety layers with confirmation prompts before deletion
- 🚀 **Fast & Efficient** - Parallel scanning with progress updates
- 📊 **Detailed Reports** - Multiple output formats (summary, table, JSON, YAML)
- 🔐 **Permission Management** - Smart sudo handling for system files with pre-flight analysis
- ⚙️ **Highly Configurable** - Customize what gets cleaned and how
- 🎯 **Targeted Cleaning** - Clean by category (cache, temp, logs, downloads, etc.)
- 🐳 **Docker Cleanup** - Clean unused Docker images, containers, volumes, and build cache
- 🔐 **Secure Deletion** - DoD 5220.22-M and Gutmann secure file wiping standards
- ⏰ **Daemon Mode** - Scheduled automated cleanups with cron-like scheduling

## 📦 Installation

### Homebrew (Recommended for macOS/Linux)

```bash
brew install fenilsonani/tidyup/tidyup
```

Or tap first:
```bash
brew tap fenilsonani/tidyup
brew install tidyup
```

### Go Install (requires Go 1.21+)

```bash
go install github.com/fenilsonani/cleanup-cache/cmd/tidyup@latest
```

### Download Binary

Download the latest release from [GitHub Releases](https://github.com/fenilsonani/cleanup-cache/releases/latest).

### From Source

```bash
git clone https://github.com/fenilsonani/cleanup-cache.git
cd cleanup-cache
make build
sudo mv bin/tidyup /usr/local/bin/
```

## 🚀 Quick Start

### 1. Scan your system
```bash
tidyup scan
```

### 2. Clean with preview (dry-run)
```bash
tidyup clean --dry-run
```

### 3. Actually clean files
```bash
tidyup clean
```

## 📖 Usage

### Commands

#### `tidyup scan`
Scan the system and report what can be cleaned without making any changes.

```bash
tidyup scan
tidyup scan --output table
tidyup scan --output json
```

#### `tidyup clean`
Clean the system based on your configuration.

```bash
tidyup clean                    # Interactive with confirmation
tidyup clean --dry-run         # Preview what will be deleted
tidyup clean --force           # Skip confirmation prompts
tidyup clean --category cache  # Clean only specific category
```

#### `tidyup report`
Generate a detailed report of cleanup opportunities.

```bash
tidyup report                           # Show summary
tidyup report --output json             # JSON format
tidyup report --output yaml             # YAML format
tidyup report --file report.json        # Save to file
```

#### `tidyup config`
Display current configuration and config file location.

```bash
tidyup config
```

### Categories

TidyUp can clean the following types of files:

- **cache** - Application caches and temporary data
- **temp** - Temporary files and directories
- **logs** - Log files and archives
- **package_managers** - Package manager caches (npm, pip, go, etc.)
- **downloads** - Files in Downloads folder older than 30 days
- **trash** - Items in system trash
- **browser_cache** - Web browser caches
- **docker** - Unused Docker containers, images, and volumes

### Configuration

The tool will work with default settings, but you can customize behavior by creating a config file at `~/.config/cleanup-cache/config.yaml`:

```yaml
# Example configuration file
# ~/.config/cleanup-cache/config.yaml

# Global settings
dry_run: false
verbose: false
min_file_age: 1  # Hours - never delete files younger than this

# Categories to include/exclude
categories:
  cache: true
  temp: true
  logs: true
  package_managers: true
  downloads: false  # Disabled by default for safety
  docker: false     # Requires Docker to be installed

# Age thresholds (in days)
age_thresholds:
  logs: 30
  downloads: 90
  temp: 7

# Exclusions
exclude_patterns:
  - "*/important/*"
  - "*.keep"
  - "*/Documents/*"

# Docker settings (only applies when docker category is enabled)
docker:
  enabled: false
  clean_images: true
  clean_containers: true
  clean_volumes: false        # Disabled - may contain important data
  clean_build_cache: true
  only_dangling_images: true  # Only clean untagged images
  only_stopped_containers: true
  image_age_days: 7
  container_age_days: 1

# Secure deletion (military-grade file wiping)
secure_deletion:
  enabled: false              # Disabled by default
  standard: "dod522022"       # "dod522022", "gutmann", "random", "none"
  verify_writes: true         # Verify each overwrite pass
  force_sync: true            # Force sync to disk

# Daemon mode for scheduled cleanups
daemon:
  enabled: false
  pid_file: "/var/run/cleanup-cache.pid"
  log_file: "/var/log/cleanup-cache.log"
  schedules:
    - name: "daily_cleanup"
      schedule: "0 2 * * *"   # Every day at 2 AM
      categories:
        cache: true
        temp: true
      dry_run: false
  notifications:
    enabled: false
    on_success: true
    on_failure: true
```

## 🛡️ Safety Features

- **Dry Run Mode** - Preview what will be deleted before actually cleaning
- **Confirmation Prompts** - Interactive confirmation before deletion
- **Permission Analysis** - Shows which files need elevated permissions
- **Smart Exclusions** - Automatically excludes important system directories
- **Size Warnings** - Warns before deleting large files

## 📊 Output Formats

### Summary Format (default)
```
📊 System Scan Results

📂 Found 1,234 files (2.3 GB)
├── 📦 Cache: 892 files (1.8 GB)
├── 🗂️  Temp Files: 312 files (450 MB)
├── 📝 Logs: 30 files (50 MB)
└── 🗑️  Trash: 0 files (0 B)
```

### Table Format
```
┌─────────────┬──────┬────────────┬─────────────────────────────────────┐
│ CATEGORY    │ COUNT│ SIZE       │ PATH                              │
├─────────────┼──────┼────────────┼─────────────────────────────────────┤
│ cache       │  892 │ 1.8 GB     │ ~/Library/Caches                  │
│ temp        │  312 │ 450 MB     │ /tmp                              │
└─────────────┴──────┴────────────┴─────────────────────────────────────┘
```

### JSON Format
```json
{
  "total_count": 1234,
  "total_size": 2345678901,
  "categories": {
    "cache": {
      "count": 892,
      "size": 1932735283,
      "paths": ["/Users/user/Library/Caches"]
    }
  }
}
```

## 🔄 Automation

### Cron Job
Set up automatic weekly cleaning:

```bash
# Edit crontab
crontab -e

# Add weekly cleanup (every Sunday at 2 AM)
0 2 * * 0 /usr/local/bin/tidyup clean --force
```

### Script Usage
Use in scripts with specific options:

```bash
#!/bin/bash
# Backup important data before cleaning
echo "Starting backup..."
./backup.sh

# Clean with verbose output
tidyup clean --force --verbose

echo "Cleanup complete!"
```

## 🐳 Docker Support

Clean Docker resources safely - only stops containers, removes unused images, and cleans build cache:

```bash
# Preview Docker cleanup
tidyup clean --category docker --dry-run

# Clean Docker resources
tidyup clean --category docker --force
```

**Docker Safety Features:**
- Only removes **stopped** containers (never running ones)
- Only removes **dangling/unused** images by default
- Volumes are disabled by default to prevent data loss
- Configurable age thresholds for images and containers

## 🔐 Secure Deletion

For sensitive data, enable secure deletion to overwrite files before removing:

```yaml
# In config.yaml
secure_deletion:
  enabled: true
  standard: "dod522022"  # DoD 5220.22-M (3 passes)
```

**Available Standards:**
- `dod522022` - DoD 5220.22-M (3 passes) - Good balance of security and speed
- `gutmann` - Gutmann method (35 passes) - Maximum security, very slow
- `random` - Random data overwrite (configurable passes)
- `none` - Standard deletion (fastest)

## ⏰ Daemon Mode

Run CleanupCache as a background service for automated scheduled cleanups:

```bash
# Start the daemon
cleanup-daemon --config ~/.config/cleanup-cache/config.yaml

# Test configuration
cleanup-daemon --test-config

# Run in foreground (for debugging)
cleanup-daemon --foreground
```

**Daemon Features:**
- Cron-style scheduling (e.g., `"0 2 * * *"` for daily at 2 AM)
- Multiple schedules with different categories
- Email and webhook notifications
- Graceful shutdown handling
- PID file management

## 🔧 Advanced Usage

### Clean Specific Categories
```bash
# Clean only cache and temp files
tidyup clean --category cache --force

# Clean multiple categories
tidyup clean --category cache --category logs --force
```

### Generate Reports for Analysis
```bash
# Generate JSON report for analysis
tidyup report --output json --file analysis.json

# Use with other tools
tidyup report --output json | jq '.total_size'
```

### Configuration Management
```bash
# Show current configuration
tidyup config

# Use custom config file
tidyup --config ~/custom-config.yaml clean
```

## 🐛 Troubleshooting

### Permission Denied
```bash
# The tool will prompt for sudo if needed
# Or run with sudo explicitly
sudo tidyup clean
```

### Files Not Deleted
Check the error output for specific reasons:

- **File in use**: Close the application using the file
- **Permission denied**: Run with sudo or check file ownership
- **System protection**: Some system files are protected

### Dry Run Shows Different Results
Files might be created/deleted between scan and actual cleaning. Always run with `--dry-run` first to see current state.

## 📝 Examples

### Before and After
```bash
$ tidyup scan
📊 System Scan Results

📂 Found 2,543 files (4.2 GB)
├── 📦 Cache: 1,892 files (3.1 GB)
├── 🗂️  Temp Files: 412 files (850 MB)
├── 📝 Logs: 239 files (250 MB)
└── 🗑️  Trash: 0 files (0 B)

$ tidyup clean --force
Scanning system...
📊 Cleanup Complete!
✅ Successfully deleted: 2,543 files (4.2 GB)
```

### Selective Cleaning
```bash
# Only clean caches, leave everything else
$ tidyup clean --category cache --dry-run
📊 System Scan Results

📂 Found 1,892 files (3.1 GB)
├── 📦 Cache: 1,892 files (3.1 GB)

$ tidyup clean --category cache --force
✅ Successfully deleted: 1,892 files (3.1 GB)
```

## 🤝 Contributing

Contributions are welcome! Please feel free to submit a Pull Request.

1. Fork the project
2. Create your feature branch (`git checkout -b feature/AmazingFeature`)
3. Commit your changes (`git commit -m 'Add some AmazingFeature'`)
4. Push to the branch (`git push origin feature/AmazingFeature`)
5. Open a Pull Request

## 📄 License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

## 🙏 Acknowledgments

- Built with [Cobra](https://github.com/spf13/cobra) for CLI framework
- Inspired by various system cleaning tools
- Thanks to all contributors

## 📞 Support

If you encounter any issues:

1. Check the [Issues](https://github.com/fenilsonani/cleanup-cache/issues) page
2. Create a new issue with details about your system
3. Include the output of `tidyup --version`

---

**⚠️ Warning**: This tool deletes files permanently. Always review what will be deleted with `--dry-run` first, especially when using `--force`.