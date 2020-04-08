#!/bin/bash

# Script to exercise building all of the DevOps images

SRCDIR="$1"
if [ -n "${SRCDIR}" ]
then
    SRCDIR="$(realpath "${SRCDIR}")"
fi

if [ -n "${CONTENT_MIRROR_BASE_URL}" ]
then
    IMAGESURL="${CONTENT_MIRROR_BASE_URL}/current/config/image/"
    CLRINST_ARGS="${CLRINST_ARGS} --swupd-url ${CONTENT_MIRROR_BASE_URL}/update/"
else
    IMAGESURL="https://download.clearlinux.org/current/config/image/"
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

    if [ "${code}" -eq 0 ]
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

echo "Downloading latest image YAML files ..." |& tee -a "${OUTPUT}"
wget -nd --cut-dirs=1 -r --no-parent \
    --reject "index.html*" -e robots=off --wait 0.5 "${IMAGESURL}" \
    |& tee -a "${OUTPUT}" | grep -E "${IMAGESURL}"
if [ "${PIPESTATUS[0]}" -ne 0 ]
then
    echo "Failed to download required files..."
    exit_with_cleanup 1
fi

# Fix execute permission on hook scripts
chmod -R +x ./*.{pl,py,sh,bash} 1>>"${OUTPUT}" 2>>"${OUTPUT}"

ls "${TMPDIR}"/ 1>>"${OUTPUT}" 2>>"${OUTPUT}"

for y in ./*.yaml
do
    log="$(basename "${y}" | sed -e 's/.yaml$/.log/')"

    /bin/rm -rf builddir |& tee -a "${OUTPUT}"
    mkdir builddir |& tee -a "${OUTPUT}"
    pushd builddir || change_dir_error "Can not pushd to builddir: $?"
    log="${PWD}/${log}"

    echo "" |& tee -a "${OUTPUT}"
    echo "Test build image ${y} with log $log ..." |& tee -a "${OUTPUT}"

    # ignore control-c during image builds
    trap "" SIGINT
    "${CLRINST}" -c "${TMPDIR}/${y}" --log-file "${log}" "${CLRINST_ARGS}" |& tee -a "${OUTPUT}"
    if [ "${PIPESTATUS[0]}" -ne 0 ]
    then
        echo "Failed to build ${y}!" |& tee -a "${OUTPUT}"
        pwd |& tee -a "${OUTPUT}"
        exit_with_cleanup 2
    else
        echo "Succeeded in building ${y}" |& tee -a "${OUTPUT}"
    fi
    echo "" |& tee -a "${OUTPUT}"

    # Catch control-c
    trap catch_int SIGINT
    sleep 1

    popd || change_dir_error "Can not popd: $?"

    if [ -f "${SAVEDIR}/.only_one" ]
    then
        echo "Found ${SAVEDIR}/.only_one file; bailing out!"
        break
    fi
done

popd || change_dir_error "Can not popd: $?"

exit_with_cleanup 0
