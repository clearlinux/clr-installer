#!/bin/bash

# c-basic-offset: 4; tab-width: 4; indent-tabs-mode: t
# vi: set shiftwidth=4 tabstop=4 noexpandtab:
# :indentSize=4:tabSize=4:noTabs=false:

# Developer Image Pre Install steps

CHROOTPATH=$1
export HOOKDIR=$(dirname $0)

LINES=$(git status --porcelain --untracked=no | wc -l)

if [ ${LINES} -ne 0 ]; then
    echo "Check-in or stash all files first!"
    echo ""
    git status --untracked=no

    exit 1
fi

exit 0

# Editor modelines  -  https://www.wireshark.org/tools/modelines.html
#
# Local variables:
# c-basic-offset: 4
# tab-width: 4
# indent-tabs-mode: t
# End:
#
# vi: set shiftwidth=4 tabstop=4 noexpandtab:
# :indentSize=4:tabSize=4:noTabs=false:
#
