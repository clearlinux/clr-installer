#!/bin/bash

shopt -s lastpipe

export CHECK_COVERAGE="true"

go clean -testcache

treechanges=$(git status -s | wc -l)
tmp=$(mktemp)

# new coverage
declare -A ncoverage

# current coverage
declare -A ccoverage

echo "saving to: $tmp"

make check 2>/dev/null 1>$tmp
if [ $? != 0 ]; then
    echo "Error on running \"make check\""
    cat $tmp
    rm $tmp
    exit 1
fi

load_coverage() {
    grep "coverage:" $1 | while read -r line ; do
        state=($(echo $line))
        file=${state[1]}
        coverage=${state[4]}
        eval $2["${file##*/}"]=${coverage%\%}
    done
}

load_coverage "$tmp" ncoverage
load_coverage "tests/coverage-curr-status" ccoverage

decreased=0

for key in ${!ccoverage[@]}; do
    ncov=${ncoverage[$key]}
    ccov=${ccoverage[$key]}

    if [[ ${ncov/.} -lt ${ccov/.} ]]; then
        echo "FATAL: decreased coverage for: $key - was: $ccov, now: $ncov"
        decreased=1
    fi
done

if [ $decreased -eq 1 ]; then
    cat $tmp
    rm $tmp
    exit 1
fi

covchanged=$(diff -u tests/coverage-curr-status $tmp | wc -l)

if [ $covchanged -gt 0 ] && [ $treechanges -eq 0 ] && [ -n "$UPDATE_COVERAGE" ]; then
    cp $tmp tests/coverage-curr-status
    git commit -a -m "updated code coverage" -s
    echo "Code coverage has increased. Database was updated."
else
    echo "Success. No coverage decreased."
fi

rm $tmp
