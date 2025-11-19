# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [0.2.0] - 2024-11-19

### Added - Interactive Mode Overhaul 🎉

#### Real-Time Progress & Feedback
- **Live scan progress** - Shows current directory being scanned, file counts, and sizes per category
- **Live cleanup progress** - Displays current file being deleted with deletion rate (files/sec) and ETA
- Progress updates every 100ms for responsive UI
- Eliminates "frozen" appearance during long operations

#### Advanced File Browser
- **Sortable columns** - Sort by name, size, date, or category (press `s`)
- **Reverse sort** - Toggle ascending/descending order (press `S`)
- **Search/Filter** - Fuzzy matching on filenames (press `/`)
- **Visual mode** - Vim-style range selection (press `v`)
- **Bulk operations menu** (press `b`):
  - Select files larger than threshold size
  - Select files older than N days
  - Select files by glob pattern
  - Invert selection
  - Clear all selections
- **Professional table layout** - Column headers with sort indicators (↑/↓)
- **Enhanced pagination** - Shows "Page 2/5 (Items 21-40 of 100)"
- **Quick selection** - Press `x` to toggle and move down

#### Navigation Improvements
- **Vim-style bindings** - `gg` (top), `G` (bottom), `j/k` (up/down)
- **Page navigation** - `Ctrl+f`/`Ctrl+b` or `PageDown`/`PageUp`
- **Smart scrolling** - Maintains context while navigating

#### UX Enhancements
- **Persistent status bar** - Always shows selection count, total size, and context-aware shortcuts
- **Risk indicators** - LOW/MEDIUM/HIGH warnings based on file count and categories
- **Enhanced confirmation view**:
  - Detailed breakdown by category with file counts and sizes
  - Risk-based default cursor position (high risk defaults to Cancel)
  - Three-button interface: [Yes, delete] [Review] [Cancel]
  - Press `e` to go back and edit selection
- **Context-aware help** - Different keyboard shortcuts shown per view
- **Responsive design** - Adapts to any terminal size (80x24 minimum)
- **Smart path truncation** - Shows start and end: `/very/long/.../end/of/path`
- **Terminal size warnings** - Alerts if terminal is too small

### Changed
- Improved error handling throughout interactive mode
- Better memory efficiency with streaming architecture
- Enhanced visual feedback with colors and formatting
- Status bar replaces static help text in most views

### Technical Improvements
- Created `internal/ui/utils/layout.go` for responsive layout utilities
- Created `internal/ui/components/statusbar.go` for reusable status bar
- Refactored all view models to support window resize events
- Added progress subscription system for real-time updates
- Implemented non-blocking goroutine-based scanning and cleanup

### Key Bindings Reference

```
Navigation:
  ↑/↓, j/k      Move up/down
  gg            Jump to first item
  G             Jump to last item
  Ctrl+f, PgDn  Page down
  Ctrl+b, PgUp  Page up

Selection:
  Space         Toggle selection
  x             Toggle + move down
  v             Visual mode (range selection)
  Ctrl+a        Select all
  Ctrl+d        Deselect all

File Browser:
  s             Cycle sort mode
  S             Reverse sort order
  /             Search/filter
  b             Bulk operations menu

General:
  Enter         Continue/confirm
  e             Edit selection (from confirmation)
  Esc           Back/cancel
  ?             Help
  q, Ctrl+c     Quit
```

## [0.1.0] - 2024-11-19

### Added
- Initial release
- Multi-platform support (macOS, Linux)
- Smart file scanning across 6 categories (cache, temp, logs, duplicates, downloads, package managers)
- Basic interactive TUI mode
- Permission management with sudo support
- Configuration system with YAML
- Multiple output formats (summary, table, JSON, YAML)
- Safety features (protected paths, age thresholds, exclude patterns)
- One-line installer script

[0.2.0]: https://github.com/fenilsonani/cleanup-cache/compare/v0.1.0...v0.2.0
[0.1.0]: https://github.com/fenilsonani/cleanup-cache/releases/tag/v0.1.0
