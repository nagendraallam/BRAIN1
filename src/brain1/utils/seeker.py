import subprocess
import sys


def search_in_compressed_files(files: list[str], query: str) -> dict[str, list[str]]:
    """
    Use ripgrep (-z) to search *query* across a list of .zst files.

    Returns a dict mapping filename → list of matching lines.
    Exits with an error message on rg failure (returncode 2).
    """
    if not files:
        return {}

    rg_cmd = [
        "rg", "-z",
        "--color=never",
        "--with-filename",
        "--no-line-number",
        "-i",
        query,
    ] + files

    result = subprocess.run(rg_cmd, capture_output=True, text=True)

    if result.returncode == 2:
        print(f"Error: ripgrep failed.\n{result.stderr.strip()}", file=sys.stderr)
        sys.exit(1)

    matches: dict[str, list[str]] = {}
    for line in result.stdout.splitlines():
        parts = line.split(":", 1)
        if len(parts) == 2:
            filepath, matched_text = parts
            import os
            filename = os.path.basename(filepath)
            matches.setdefault(filename, []).append(matched_text.strip())

    return matches
