# CleanupCache 🗑️

A powerful, safe, and intelligent CLI tool to clean up your Mac or Linux system by removing unnecessary files, caches, logs, and more.

## ✨ Features

- 🔍 **Smart Scanning** - Intelligently identifies cleanable files across multiple categories with real-time progress
- 🎯 **Advanced Interactive Mode** - Professional TUI with sorting, filtering, search, bulk operations, and visual selection
- 🔒 **Safe & Secure** - Multiple safety layers with risk indicators (LOW/MEDIUM/HIGH) before deletion
- 🚀 **Fast & Efficient** - Parallel scanning with live progress updates and ETA
- 📊 **Detailed Reports** - Multiple output formats (summary, table, JSON, YAML)
- 🔐 **Permission Management** - Smart sudo handling for system files with pre-flight analysis
- ⚙️ **Highly Configurable** - Customize what gets cleaned and how
- 📱 **Responsive Design** - Adapts to any terminal size with smart content truncation
- ⌨️ **Vim-Style Navigation** - Familiar key bindings (gg, G, j/k, visual mode)

## 📦 Installation

### Quick Install (Recommended)

```bash
curl -sSL https://raw.githubusercontent.com/fenilsonani/cleanup-cache/main/install.sh | bash
```

### Manual Installation

#### From Source (requires Go 1.21+)

```bash
# Clone the repository
git clone https://github.com/fenilsonani/cleanup-cache.git
cd cleanup-cache

# Build and install
go build -o cleanup ./cmd/cleanup
sudo mv cleanup /usr/local/bin/

# Create config directory
mkdir -p ~/.config/cleanup-cache
```

## 🚀 Quick Start

### 1. Scan your system
```bash
cleanup scan
```

### 2. Interactive mode (recommended for first-time users)
```bash
cleanup interactive
```

### 3. Clean with preview (dry-run)
```bash
cleanup clean --dry-run
```

### 4. Actually clean files
```bash
cleanup clean
```

## 📖 Usage

### Commands

#### `cleanup scan`
Scan the system and report what can be cleaned without making any changes.

```bash
cleanup scan
cleanup scan --output table
cleanup scan --output json
```

#### `cleanup interactive`
Launch interactive TUI mode with advanced file browser and selection.

```bash
cleanup interactive
cleanup interactive --dry-run    # Preview mode
```

**Interactive Mode Features:**
- 🔄 **Real-time progress** - Live updates during scan and cleanup with ETA
- 📊 **Advanced file browser** - Sort, filter, and search through files
- 🎨 **Professional table layout** - Column headers with sort indicators
- 🎯 **Visual mode** - Select ranges of files vim-style
- ⚡ **Bulk operations** - Select files by size, age, or pattern
- 📱 **Responsive design** - Adapts to any terminal size (80x24+)
- ⚠️ **Risk indicators** - LOW/MEDIUM/HIGH warnings before deletion
- 📍 **Status bar** - Always shows selection count and shortcuts

**Key Bindings:**
```
Navigation:
  ↑/↓, j/k      Move up/down
  gg            Jump to first item
  G             Jump to last item
  Ctrl+f, PgDn  Page down
  Ctrl+b, PgUp  Page up

Selection:
  Space         Toggle selection
  x             Toggle + move down (quick select)
  v             Visual mode (range selection)
  Ctrl+a        Select all
  Ctrl+d        Deselect all

File Browser:
  s             Cycle sort (name/size/date/category)
  S             Reverse sort order
  /             Search/filter (fuzzy matching)
  b             Bulk operations menu

General:
  Enter         Continue to next screen
  e             Edit selection (from confirmation)
  Esc           Back/Cancel
  ?             Help
  q, Ctrl+c     Quit
```

#### `cleanup clean`
Clean the system based on configuration.

```bash
cleanup clean                      # Clean everything
cleanup clean --dry-run            # Preview only
cleanup clean --category cache     # Clean specific category
cleanup clean --force              # Skip confirmation
```

#### `cleanup report`
Generate detailed reports in various formats.

```bash
cleanup report
cleanup report --output json
cleanup report --output yaml --file report.yaml
```

#### `cleanup config`
Display current configuration.

```bash
cleanup config
```

### Categories

The tool cleans files in the following categories:

| Category | Description | Default |
|----------|-------------|---------|
| **cache** | Browser caches, app caches, system caches | ✅ Enabled |
| **temp** | Temporary files | ✅ Enabled |
| **logs** | Log files older than threshold | ✅ Enabled |
| **package_managers** | Brew, npm, pip, apt caches | ✅ Enabled |
| **duplicates** | Duplicate files (by hash) | ❌ Disabled |
| **downloads** | Old files in Downloads folder | ❌ Disabled |

## ⚙️ Configuration

Configuration file location: `~/.config/cleanup-cache/config.yaml`

### Default Configuration

```yaml
categories:
  cache: true
  temp: true
  logs: true
  duplicates: false          # Disabled - can be aggressive
  downloads: false           # Disabled - safety first
  package_managers: true

age_thresholds:
  logs: 30                   # Clean logs older than 30 days
  downloads: 90              # Clean downloads older than 90 days
  temp: 7                    # Clean temp files older than 7 days

size_limits:
  min_file_size: "1KB"       # Ignore tiny files
  max_file_size: "10GB"      # Skip huge files (safety)

exclude_patterns:
  - "*/important/*"
  - "*.keep"
  - "*/Documents/*"
  - "*/Pictures/*"
  - "*/Music/*"
  - "*/Videos/*"

dry_run: false               # Actually delete files
min_file_age: 1              # Never delete files < 1 hour old
verbose: false
```

### Edit Configuration

```bash
nano ~/.config/cleanup-cache/config.yaml
```

## 🛡️ Safety Features

1. **Protected Paths** - Critical system directories are always protected
2. **Symlink Resolution** - Prevents traversal attacks
3. **Age Thresholds** - Never deletes recently created files
4. **Confirmation Prompts** - Double-checks before deletion
5. **Dry-Run Mode** - Preview changes before applying
6. **Permission Analysis** - Smart sudo handling
7. **Special File Detection** - Skips devices, sockets, pipes

### Always Protected Paths
- `/` (root)
- `/System`, `/Applications`, `/Library/System` (macOS)
- `/usr`, `/bin`, `/sbin`, `/etc` (system binaries and configs)
- `/var`, `/boot`, `/dev`, `/proc`, `/sys` (system directories)

## 🎯 Examples

### Safe Exploration
```bash
# Scan and see what can be cleaned
cleanup scan

# Interactive mode - browse and select files
cleanup interactive

# Preview what would be deleted
cleanup clean --dry-run
```

### Targeted Cleaning
```bash
# Clean only cache files
cleanup clean --category cache

# Clean logs with preview
cleanup clean --category logs --dry-run

# Clean with confirmation
cleanup clean
```

### Generate Reports
```bash
# Summary report
cleanup report

# Detailed table view
cleanup report --output table

# JSON export
cleanup report --output json --file cleanup-report.json
```

## 📊 Output Formats

The tool supports multiple output formats:

- **summary** - Human-readable summary (default)
- **table** - Detailed table view
- **json** - Machine-readable JSON
- **yaml** - YAML format

## 🔧 Advanced Usage

### Custom Config File
```bash
cleanup clean --config /path/to/custom-config.yaml
```

### Verbose Output
```bash
cleanup scan --verbose
```

### Force Mode (No Prompts)
```bash
cleanup clean --force
```

## 🐛 Troubleshooting

### Permission Issues
Some files require sudo. The tool will:
1. Analyze permissions first
2. Prompt for password if needed
3. Use sudo only for files that require it

### Config Not Found
```bash
# View current config location
cleanup config

# Create default config
mkdir -p ~/.config/cleanup-cache
```

### Build from Source Issues
```bash
# Ensure Go 1.21+ is installed
go version

# Clean build
go clean -cache
go build -o cleanup ./cmd/cleanup
```

## 📝 License

MIT License - See LICENSE file for details

## 🤝 Contributing

Contributions are welcome! Please feel free to submit issues or pull requests.

## 📬 Support

- **Issues**: [GitHub Issues](https://github.com/fenilsonani/cleanup-cache/issues)
- **Discussions**: [GitHub Discussions](https://github.com/fenilsonani/cleanup-cache/discussions)

## ⚠️ Disclaimer

While this tool includes multiple safety features, **always review what will be deleted** before proceeding. Use `--dry-run` mode first to preview changes. The authors are not responsible for any data loss.

---

Made with ❤️ by [Fenil Sonani](https://github.com/fenilsonani)
