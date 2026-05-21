import zstandard as zstd
import os

def compress_file(file_path: str, force: bool = False) -> str:
    """
    Compress a file using zstandard
    Pr stats of compression
    """
    output_file_path = file_path + ".zstd"
    # TODO: Make level and threads configurable
    compressor = zstd.ZstdCompressor(level=9, threads=os.cpu_count())

    if os.path.exists(output_file_path) and not force:
        print(f"File {output_file_path} already exists, skipping compression")
        print(f"If this is an ERROR, run with force=True")
        return output_file_path
   
    with open(file_path, "rb") as f:
        data = f.read()
    compressed_data = compressor.compress(data)
    with open(output_file_path, "wb") as f:
        f.write(compressed_data)
    print(f"Compressed {file_path} to {output_file_path}")
    print(f"Compression ratio: {len(compressed_data) / len(data)}")
    return output_file_path
