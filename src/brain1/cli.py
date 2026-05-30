import argparse
import os
import sys
import time
import subprocess
import zstandard as zstd

BRAIN1_DIR = os.path.expanduser("~/.config/brain1")
FILE_EXT = ".md.zst"


def ensure_dir() -> None:
    os.makedirs(BRAIN1_DIR, exist_ok=True)


def check_ripgrep() -> None:
    result = subprocess.run(["which", "rg"], capture_output=True)
    if result.returncode != 0:
        print(
            "Error: ripgrep (rg) is not installed or not in PATH.\n"
            "Install it via:\n"
            "  macOS:   brew install ripgrep\n"
            "  Linux:   apt install ripgrep  /  cargo install ripgrep\n"
            "  Windows: scoop install ripgrep\n"
            "  or:      pip install ripgrep-rs",
            file=sys.stderr,
        )
        sys.exit(1)


def sanitize_name(name: str) -> str:
    """Convert a user-supplied name to a safe filename stem."""
    safe = name.strip().replace(" ", "_")
    safe = "".join(c for c in safe if c.isalnum() or c in ("_", "-"))
    return safe or "entry"


def unique_filepath(stem: str) -> str:
    """Return a path that does not yet exist, adding _2, _3 … on collision."""
    candidate = os.path.join(BRAIN1_DIR, stem + FILE_EXT)
    if not os.path.exists(candidate):
        return candidate
    counter = 2
    while True:
        candidate = os.path.join(BRAIN1_DIR, f"{stem}_{counter}{FILE_EXT}")
        if not os.path.exists(candidate):
            return candidate
        counter += 1


def compress_to_file(content: str, filepath: str) -> None:
    compressor = zstd.ZstdCompressor(level=9, threads=os.cpu_count() or 1)
    with open(filepath, "wb") as fh:
        fh.write(compressor.compress(content.encode("utf-8")))


def decompress_from_file(filepath: str) -> str:
    decompressor = zstd.ZstdDecompressor()
    with open(filepath, "rb") as fh:
        data = fh.read()
    return decompressor.decompress(data).decode("utf-8")


def all_entries() -> list[str]:
    """Return sorted list of absolute paths for all stored entries."""
    try:
        files = sorted(
            os.path.join(BRAIN1_DIR, f)
            for f in os.listdir(BRAIN1_DIR)
            if f.endswith(FILE_EXT)
        )
    except FileNotFoundError:
        files = []
    return files


# ---------------------------------------------------------------------------
# Commands
# ---------------------------------------------------------------------------

def cmd_add(args: argparse.Namespace) -> None:
    ensure_dir()

    if args.text:
        content = " ".join(args.text)
    else:
        if sys.stdin.isatty():
            print("Enter text (press Ctrl+D when done):")
        content = sys.stdin.read()

    if not content.strip():
        print("Error: No content provided. Pass text as an argument or pipe it via stdin.", file=sys.stderr)
        sys.exit(1)

    if args.name:
        stem = sanitize_name(args.name)
    else:
        stem = str(int(time.time() * 1000))

    filepath = unique_filepath(stem)

    try:
        compress_to_file(content, filepath)
    except OSError as exc:
        print(f"Error: Could not write file '{filepath}': {exc}", file=sys.stderr)
        sys.exit(1)

    filename = os.path.basename(filepath)
    size = os.path.getsize(filepath)
    print(f"Saved  →  {filename}  ({size} bytes compressed)")


def cmd_find(args: argparse.Namespace) -> None:
    ensure_dir()
    check_ripgrep()

    entries = all_entries()
    if not entries:
        print("No entries stored yet. Add some with:  brain1 add <text>")
        return

    query = args.query

    rg_cmd = [
        "rg", "-z",
        "--color=never",
        "--with-filename",
        "--no-line-number",
        "-i",
        query,
    ] + entries

    result = subprocess.run(rg_cmd, capture_output=True, text=True)

    if result.returncode == 2:
        print(f"Error: ripgrep failed.\n{result.stderr.strip()}", file=sys.stderr)
        sys.exit(1)

    if result.returncode == 1 or not result.stdout.strip():
        print(f"No matches found for '{query}'.")
        return

    # rg output format: /absolute/path/to/file.md.zst:matched line
    matches: dict[str, list[str]] = {}
    for line in result.stdout.splitlines():
        # Split on first ":" only — absolute paths on macOS/Linux never contain ":"
        parts = line.split(":", 1)
        if len(parts) == 2:
            filepath, matched_text = parts
            filename = os.path.basename(filepath)
            matches.setdefault(filename, []).append(matched_text.strip())

    print(f"Found '{query}' in {len(matches)} file(s):\n")
    for filename, lines in matches.items():
        print(f"  {filename}")
        for ln in lines:
            print(f"    > {ln}")
    print()


def cmd_read(args: argparse.Namespace) -> None:
    ensure_dir()
    name = args.name

    # Match strategy: exact filename → stem+ext → prefix match
    candidates: list[str] = []

    exact = os.path.join(BRAIN1_DIR, name)
    if os.path.exists(exact) and name.endswith(FILE_EXT):
        candidates.append(exact)

    with_ext = os.path.join(BRAIN1_DIR, name + FILE_EXT)
    if os.path.exists(with_ext):
        candidates.append(with_ext)

    if not candidates:
        for entry in all_entries():
            basename = os.path.basename(entry)
            stem = basename[: -len(FILE_EXT)]
            if stem.startswith(name) or basename.startswith(name):
                candidates.append(entry)

    # Deduplicate while preserving order
    seen: set[str] = set()
    unique_candidates = [c for c in candidates if not (c in seen or seen.add(c))]  # type: ignore[func-returns-value]

    if not unique_candidates:
        print(f"Error: No entry found matching '{name}'.", file=sys.stderr)
        print("Use 'brain1 list' to see all stored entries.", file=sys.stderr)
        sys.exit(1)

    if len(unique_candidates) > 1:
        print(f"Multiple entries match '{name}':")
        for c in unique_candidates:
            print(f"  {os.path.basename(c)}")
        print("Please be more specific.", file=sys.stderr)
        sys.exit(1)

    filepath = unique_candidates[0]
    try:
        content = decompress_from_file(filepath)
    except Exception as exc:
        print(f"Error: Could not read '{os.path.basename(filepath)}': {exc}", file=sys.stderr)
        sys.exit(1)

    print(f"=== {os.path.basename(filepath)} ===\n")
    print(content)


def cmd_list(args: argparse.Namespace) -> None:  # noqa: ARG001
    ensure_dir()
    entries = all_entries()

    if not entries:
        print("No entries stored yet. Add some with:  brain1 add <text>")
        return

    print(f"Stored entries ({len(entries)}):\n")
    for filepath in entries:
        filename = os.path.basename(filepath)
        size = os.path.getsize(filepath)
        print(f"  {filename:<50}  {size} bytes")
    print()


# ---------------------------------------------------------------------------
# Entry point
# ---------------------------------------------------------------------------

def main() -> int:
    parser = argparse.ArgumentParser(
        prog="brain1",
        description="BRAIN1 — personal knowledge store: add notes, find them later.",
        formatter_class=argparse.RawDescriptionHelpFormatter,
        epilog=(
            "Examples:\n"
            "  brain1 add \"Quick thought about the project\"\n"
            "  brain1 add \"Meeting notes\" --name meeting_2026\n"
            "  echo \"Piped text\" | brain1 add\n"
            "  brain1 find \"project\"\n"
            "  brain1 read meeting_2026\n"
            "  brain1 list\n"
        ),
    )

    sub = parser.add_subparsers(dest="command", required=True, metavar="COMMAND")

    add_p = sub.add_parser("add", help="Add text to brain1 storage")
    add_p.add_argument("text", nargs="*", help="Text to store (omit to read from stdin)")
    add_p.add_argument(
        "--name", "-n",
        metavar="NAME",
        help="Custom name for the entry (default: timestamp in milliseconds)",
    )

    find_p = sub.add_parser("find", help="Search stored entries for a phrase")
    find_p.add_argument("query", help="Phrase or word to search for (case-insensitive)")

    read_p = sub.add_parser("read", help="Read and display a stored entry in full")
    read_p.add_argument("name", help="Entry name or filename (with or without extension)")

    sub.add_parser("list", help="List all stored entries")

    args = parser.parse_args()

    dispatch = {
        "add": cmd_add,
        "find": cmd_find,
        "read": cmd_read,
        "list": cmd_list,
    }

    try:
        dispatch[args.command](args)
    except KeyboardInterrupt:
        print("\nAborted.", file=sys.stderr)
        return 130

    return 0
