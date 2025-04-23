 #!/usr/bin/env bash

echo ./file-stats input.txt
./file-stats input.txt 
echo 

echo ./file-stats input.txt output.txt
./file-stats input.txt output.txt
ls -la output.txt
echo

echo ./file-stats -l -w input.txt
./file-stats -l -w input.txt 
echo

echo ./file-stats --chars input.txt -o results.txt
./file-stats --chars input.txt -o results.txt 
ls results.txt 
echo

echo ./file-stats -v input.txt
./file-stats -v input.txt
echo

echo ./file-stats
./file-stats 
echo

echo ./file-stats -h
./file-stats -h

