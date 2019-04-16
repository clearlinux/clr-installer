#!/bin/bash

OUTPUT=$(mktemp)

# Change to the top level git directory to get all of the source files
cd $(git rev-parse --show-toplevel)

# List all of the go source files (committed)
for file in $(git ls-files | grep -v '^vendor' | grep -v '^tests' | grep '.go$')
do
    # echo "$file"
    # append all of the imports for the go file, one per line to the composite file
    go list -f '{{ join .Imports "\n" }}' $file >> ${OUTPUT}
done

# Remove the imports for our own code
grep -v '^github.com/clearlinux/clr-installer'  ${OUTPUT} | sort | uniq

rm -f ${OUTPUT}

exit 0
