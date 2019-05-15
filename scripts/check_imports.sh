#!/bin/bash

INPUT=$1
OUTPUT=$(mktemp)

if [ "${INPUT}" != "" ]
then
    if [ ! -f ${INPUT} ]
    then
        echo "Input CSV file '${INPUT}' do not exist"
    exit 1
fi
fi

# Change to the top level git directory to get all of the source files
cd $(git rev-parse --show-toplevel)

# List all of the go source files (committed)
for file in $(git ls-files | grep -v '^vendor' | grep -v '^tests' | grep '.go$')
do
    # echo "$file"
    # append all of the imports for the go file, one per line to the composite file
    go list -f '{{ join .Imports "\n" }}' $file >> ${OUTPUT}
done

CSV="clr-installer-packages.csv"
if [ -f ${CSV} ]
then
    mv ${CSV} ${CSV}.$(date +%Y%m%d_%H%M)
fi

# Remove the imports for our own code
for package in $(grep -v '^github.com/clearlinux/clr-installer'  ${OUTPUT} |\
    sort | uniq | gawk -F/ '$1 ~ /\./ {print $0}')
do
    echo -n "Checking package ${package} ...  "
    if [ -f ${INPUT} ]
    then
        line=$(grep "^${package}," ${INPUT} | head -1)
        if [ "${line}" != "" ]
        then
            echo "Found in ${INPUT}"
            echo "${line}" >> ${CSV}
            continue
        fi
    fi

    tryPackage=${package}
    while [ "${tryPackage}" != "" ]
    do
        curl -f --fail --url https://${tryPackage} --output /dev/null --silent
        if [ $? == 0 ]
        then
            echo "is New"
            echo "${line}" >> ${CSV}
            echo "${package},https://${tryPackage}" >> ${CSV}
            break
        fi

        tryPackage=$(dirname ${tryPackage})
    done
done

echo "Results: ${CSV}"

rm -f ${OUTPUT}

exit 0
