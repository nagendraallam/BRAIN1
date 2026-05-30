import os
import zstandard as zstd


def compress_file(source_path: str, force: bool = False) -> str:
    """Compress *source_path* with zstandard and return the output path (.zst)."""
    output_path = source_path + ".zst"

    if os.path.exists(output_path) and not force:
        raise FileExistsError(
            f"Compressed file already exists: {output_path}. "
            "Pass force=True to overwrite."
        )

    compressor = zstd.ZstdCompressor(level=9, threads=os.cpu_count() or 1)
    with open(source_path, "rb") as fh:
        data = fh.read()

    compressed = compressor.compress(data)

    with open(output_path, "wb") as fh:
        fh.write(compressed)

    return output_path


def decompress_file(source_path: str) -> bytes:
    """Return the raw decompressed bytes of a .zst file."""
    decompressor = zstd.ZstdDecompressor()
    with open(source_path, "rb") as fh:
        return decompressor.decompress(fh.read())
