#!/bin/bash

LINES=$(git status --porcelain --untracked=no | wc -l)

if [ ${LINES} -ne 0 ]; then
    echo "Check-in or stash all files first!"
    echo ""
    git status --untracked=no

    exit 1
fi


exit 0
