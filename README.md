<div align="center">
  <img src="assets/brain1.png" alt="brain1 logo" width="160" />
  <br /><br />
  <img src="assets/banner.png" alt="brain1 banner" width="720" />
  <br /><br />
  <p>
    <b>A persistent memory layer for AI agents and developers.</b><br />
    Store thoughts, notes, and task history — compressed, searchable, instant.
  </p>
  <p>
    <img src="https://img.shields.io/badge/python-3.12%2B-blue?style=flat-square" />
    <img src="https://img.shields.io/badge/compression-zstandard-informational?style=flat-square" />
    <img src="https://img.shields.io/badge/search-ripgrep-orange?style=flat-square" />
    <img src="https://img.shields.io/badge/license-MIT-green?style=flat-square" />
  </p>
</div>

---

## What is brain1?

AI agents are stateless by default. Every new session starts cold — no memory of previous tasks, decisions, or context.

**brain1** fixes that. It gives agents (and humans) a dead-simple way to write thoughts and notes to disk in compressed form, and retrieve them later with full-text search. Think of it as a personal, offline, zero-latency knowledge store that lives in `~/.config/brain1`.

- An agent finishes a task → stores a summary with `brain1 add`
- A new session begins → searches past work with `brain1 find`
- Needs the full context of an old entry → reads it back with `brain1 read`

No databases. No servers. Just fast, compressed files on disk.

---

## Requirements

- Python 3.12+
- [ripgrep](https://github.com/BurntSushi/ripgrep) (`rg`) — for full-text search

```bash
# macOS
brew install ripgrep

# Ubuntu / Debian
apt install ripgrep

# or via pip
pip install ripgrep-rs
```

---

## Installation

```bash
# Clone the repo
git clone https://github.com/your-username/brain1.git
cd brain1

# Install (editable mode recommended for development)
uv pip install -e .

# or with pip
pip install -e .
```

Once installed, the `brain1` command is available globally.

---

## Usage

### Add an entry

```bash
# Inline text
brain1 add "Finished refactoring the auth module. Moved JWT logic to utils/auth.py."

# With a custom name
brain1 add "Deployed v2.1 to staging. All tests passing." --name deploy_notes

# Pipe from stdin
echo "Database migration complete. Rolled back index on users table." | brain1 add
```

Entries are stored as `<name>.md.zst` (or `<timestamp_ms>.md.zst` when no name is given).  
Name collisions are handled automatically: `deploy_notes_2.md.zst`, `deploy_notes_3.md.zst`, …

---

### Search across all entries

```bash
brain1 find "auth module"
```

```
Found 'auth module' in 1 file(s):

  1748123456789.md.zst
    > Finished refactoring the auth module. Moved JWT logic to utils/auth.py.
```

Search is **case-insensitive** and powered by ripgrep over zstd-compressed files.

---

### Read an entry in full

```bash
brain1 read deploy_notes
```

```
=== deploy_notes.md.zst ===

Deployed v2.1 to staging. All tests passing.
```

You can pass the name with or without the `.md.zst` extension.

---

### List all entries

```bash
brain1 list
```

```
Stored entries (3):

  1748123456789.md.zst                                71 bytes
  deploy_notes.md.zst                                 48 bytes
  deploy_notes_2.md.zst                               52 bytes
```

---

## Command Reference

| Command | Description |
|---|---|
| `brain1 add [text] [--name NAME]` | Store text (inline or from stdin) |
| `brain1 find <query>` | Full-text search across all entries |
| `brain1 read <name>` | Print a stored entry in full |
| `brain1 list` | List all stored entries |
| `brain1 --help` | Show help |
| `brain1 <command> --help` | Show help for a specific command |

---

## Storage

All entries live in `~/.config/brain1/` as `.md.zst` files (zstandard-compressed markdown).  
Nothing is sent anywhere. Everything is local.

---

<div align="center">
  <sub>built with ripgrep + zstandard · store. compress. search. think.</sub>
</div>
