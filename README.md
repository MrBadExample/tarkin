# tarkin

Terminal task manager with a kanban board, ideas capture, and activity log.

## Requirements

- Go 1.22+
- gcc (for sqlite3)

```bash
# Fedora / RHEL
sudo dnf install golang gcc

# Debian / Ubuntu
sudo apt install golang gcc
```

## Install

```bash
git clone https://github.com/MrBadExample/tarkin
cd tarkin
go build -o tarkin .
sudo mv tarkin /usr/local/bin/
```

## Usage

```bash
tarkin
```

Launches the TUI. All data is stored locally at `~/.tarkin/tarkin.db`.

## Keybindings

| Key | Action |
|-----|--------|
| `1` `2` `3` `4` | Switch views (board / ideas / log / trash) |
| `j` / `k` | Move down / up |
| `h` / `l` | Move between columns (board) · cycle value (detail) |
| `↵` | Open item · edit field · confirm |
| `esc` | Back / cancel |
| `a` | Add task or idea |
| `x` | Move to trash |
| `r` | Restore from trash · refresh |
| `X` | Delete forever (trash only) |
| `?` | Context help |
| `q` | Quit |
