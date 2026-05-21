from .utils.printer import print_hello
import glob
from .utils.compresser import compress_file
from .utils.seeker import search_file, search_compressed_file
from .utils.writer import writer
import argparse
import time
import os
import sys


def main():
    """
    Take in arguments from the command line for file path and force flag
    """
    print_hello()
    
    # check if brain1 config folder is created for user, if not create it 
    config_folder = os.path.expanduser("~/.config/brain1")
    if not os.path.exists(config_folder):
        os.makedirs(config_folder)
    
    parser = argparse.ArgumentParser(description="BRAIN1: A simple file compressor and searcher")
    sub_parser = parser.add_subparsers(dest="command", required=True)

    # write command
    sub_parser.add_parser("write",help="store string into a file") 

    # find command
    sub_parser.add_parser("find",help="search for a query in the compressed files")


    args = parser.parse_args()
    print(f"Running command: {args.command}")
    print(f"parser args: {args}")

    if args.command == "write":
        # create this file in the config_folder
        temp_file = os.path.join(config_folder, f"temp_{int(time.time())}.txt")

        content = sys.stdin.read()
        file_name = writer(temp_file, content)
        print(f"Written content to {temp_file}")
        compressed_file = compress_file(temp_file)
        #delete the temp tempo
        os.remove(temp_file)

    elif args.command == "find":
        query = input("Enter your search query: ")
        
        # get all files from the config_folder
        files = os.path.join(config_folder, "*")
        results = []
        for file in glob.glob(files):
            if file.endswith(".zstd"):
                results.extend(search_compressed_file(file, query))


if __name__ == "__main__":
    main()
