import os

def writer(file_path: str, content: str) -> str:
    """
    Write a string into a file and return the file name
    """
    with open(file_path, "w") as f:
        f.write(content)
    file_name = os.path.basename(file_path)

    return file_name
