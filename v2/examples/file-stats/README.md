## How to Build and Run:

Build: go build -o file-stats filestats.go

Run examples:

    ./file-stats input.txt (Counts all, output to stdout)

    ./file-stats input.txt output.txt (Counts all, output to file)

    ./file-stats -l -w input.txt (Counts lines and words, output to stdout)

    ./file-stats --chars input.txt -o results.txt (Counts chars, output to results.txt)

    ./file-stats -v input.txt (Verbose output)

    ./file-stats (Should fail and print usage because input file is required)

    ./file-stats -h (Or similar, should show help - implicitly via parse failure)#