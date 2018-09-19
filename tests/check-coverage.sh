#!/bin/bash

shopt -s lastpipe

go clean -testcache

tmp=$(mktemp)

# new coverage
declare -A ncoverage

# current coverage
declare -A ccoverage

make check > $tmp

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

for key in ${!ccoverage[@]}; do
    ncov=${ncoverage[$key]}
    ccov=${ccoverage[$key]}

    if [[ ${ncov/.} < ${ccov/.} ]]; then
        echo "FATAL: decreased coverage for: $key - was: $ccov, now: $ncov"
        rm $tmp
        exit 1
    fi
done

echo "Success. No coverage decreased."
rm $tmp
