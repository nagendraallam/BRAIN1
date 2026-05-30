import os


def write_text_file(filepath: str, content: str) -> str:
    """Write *content* to *filepath* and return the basename."""
    with open(filepath, "w", encoding="utf-8") as fh:
        fh.write(content)
    return os.path.basename(filepath)
