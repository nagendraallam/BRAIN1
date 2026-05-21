from ripgrep_rs import search_structured
import zstandard as zstd
import os

def search_file(file_path: str, query: str) -> list[dict]:
    """
    Search a file for a query using ripgrep
    """
    return search_structured([query], paths=[file_path], max_total=1000)

# seek in compressed file
def search_compressed_file(compressed_file_path: str, query: str) -> list[dict]:
    """
    Search a compressed file for a query using ripgrep
    """
    with open(compressed_file_path, "rb") as f:
        data = f.read()
    decompressor = zstd.ZstdDecompressor()
    decompressed_data = decompressor.decompress(data)

    with open("decompressed_data.txt", "wb") as f:
        f.write(decompressed_data)

        
    matches = search_structured([query], paths=["decompressed_data.txt"], max_total=1000)

    print(f"Found {len(matches)} matches in {compressed_file_path}")
    os.remove("decompressed_data.txt")  
    return matches
