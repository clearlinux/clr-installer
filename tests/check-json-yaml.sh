#!/bin/bash

# Script to exercise converting all of the JSON ister file
# which get converted to YAML

SRCDIR="$1"
if [ -n "${SRCDIR}" ]
then
    SRCDIR="$(realpath "${SRCDIR}")"
fi

SAVEDIR=$(pwd)
TMPDIR=$(mktemp -d)

OUTPUT="$(pwd)/$(basename "$0" | sed -e 's/.sh$/.log/')"
if [ -e "${OUTPUT}" ]
then
    mv -f "${OUTPUT}" "${OUTPUT}".last
fi

# Catch control-c
trap catch_int SIGINT

catch_int() {
    exit_with_cleanup "$((128 + 2))"
}

change_dir_error() {
    echo "$*"
    exit 1
}


exit_with_cleanup() {
    local code="$1"

    cd "${SAVEDIR}" || change_dir_error "Can not change directory to ${SAVEDIR}: $?"

    if [ "${code}" -eq 0 ] && [ ! -f "${SAVEDIR}/.no_cleanup" ]
    then
        /bin/rm -rf "${TMPDIR}"
    else
        echo ""
        echo ""
        echo "Please review and clean-up ${TMPDIR}"
    fi

    exit "${code}"
}

if [ "${EUID}" -ne 0 ]
then
    echo "This must run as root!"
    exit_with_cleanup 1
fi

if [ -n "${SRCDIR}" ] && [ -d "${SRCDIR}" ]
then
    echo "Ensure all files are committed ..." |& tee -a "${OUTPUT}"
    "${SRCDIR}"/scripts/developer-image-pre.sh |& tee -a "${OUTPUT}"
    if [ "${PIPESTATUS[0]}" -ne 0 ]
    then
        exit_with_cleanup "$?"
    fi
fi

echo "Changing to ${TMPDIR}" |& tee -a "${OUTPUT}"
pushd "${TMPDIR}" || change_dir_error "Can not pushd to ${TMPDIR}: $?"

if [ -n "${SRCDIR}" ] && [ -d "${SRCDIR}" ]
then
    echo "Cloning sources ..." |& tee -a "${OUTPUT}"
    git clone "${SRCDIR}" |& tee -a "${OUTPUT}"
    if [ "${PIPESTATUS[0]}" -ne 0 ]
    then
        exit_with_cleanup "$?"
    fi

    pushd "$(basename "${SRCDIR}")" || change_dir_error "Can not pushd: $?"
    echo "Building from sources ..." |& tee -a "${OUTPUT}"
    make build-tui 1>/dev/null 2>/dev/null
    if [ "${PIPESTATUS[0]}" -ne 0 ]
    then
        exit_with_cleanup "$?"
    fi
    CLRINST="$(pwd)"/.gopath/bin/clr-installer-tui
    popd || change_dir_error "Can not popd: $?"
else
    echo "Testing with default /usr/bin/clr-installer ..." |& tee -a "${OUTPUT}"
    CLRINST="/usr/bin/clr-installer"
fi

if [ ! -x "${CLRINST}" ]
then
    echo "Missing ${CLRINST}!" |& tee -a "${OUTPUT}"
    exit_with_cleanup 1
fi

echo "" |& tee -a "${OUTPUT}"
"${CLRINST}" --version |& tee -a "${OUTPUT}"
echo "" |& tee -a "${OUTPUT}"

echo "For a detailed log see ${OUTPUT}."
echo ""

if [ -n "${SRCDIR}" ] && [ -d "${SRCDIR}" ]
then
    JSONDIR="${TMPDIR}/clr-installer/tests/"
else
    JSONDIR="${TMPDIR}"/tests
    cp -a "${SAVEDIR}"/tests "${JSONDIR}"
fi
echo "JSON files from ${JSONDIR}" |& tee -a "${OUTPUT}"

ls "${JSONDIR}"/ 1>>"${OUTPUT}" 2>>"${OUTPUT}"

for j in "${JSONDIR}"/*.json
do
    echo "${j}" | grep -E 'invalid-' 1> /dev/null 2>/dev/null
    if [ "$?" -eq 0 ]
    then
        echo "Skipping invalid JSON ${j} ..." |& tee -a "${OUTPUT}"
        continue
    fi

    log="$(basename "${j}" | sed -e 's/.json$/.log/')"
    log="${PWD}/${log}"

    echo "" |& tee -a "${OUTPUT}"
    echo "Converting ${j} with log $log ..." |& tee -a "${OUTPUT}"

    # ignore control-c during image builds
    echo "Running "${CLRINST}" --json-yaml "${j}" --log-file="${log}"" |& tee -a "${OUTPUT}"
    trap "" SIGINT
    "${CLRINST}" --json-yaml "${j}" --log-file="${log}" |& tee -a "${OUTPUT}"
    if [ "${PIPESTATUS[0]}" -ne 0 ]
    then
        echo "Failed to convert ${j}!" |& tee -a "${OUTPUT}"
        pwd |& tee -a "${OUTPUT}"
        exit_with_cleanup 2
    else
        echo "Succeeded in converting ${j}" |& tee -a "${OUTPUT}"
    fi
    echo "" |& tee -a "${OUTPUT}"

    # Catch control-c
    trap catch_int SIGINT
    sleep 1

    if [ -f "${SAVEDIR}/.only_one" ]
    then
        echo "Found ${SAVEDIR}/.only_one file; bailing out!"
        break
    fi
done

popd || change_dir_error "Can not popd: $?"

exit_with_cleanup 0
