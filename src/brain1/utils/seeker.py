from ripgrep_rs import search_structured
import zstandard as zstd
import os
import subprocess


# seek in compressed file
def search_compressed_file(compressed_file_path: str, query: str) -> list[dict]:
    """
    Search a compressed file for a query using ripgrep
    """
    result = subprocess.run(["rg", "-z", query, compressed_file_path], capture_output=True, text=True)
    if result.returncode == 0:
        print("Found in file " + compressed_file_path + ": " + result.stdout)


