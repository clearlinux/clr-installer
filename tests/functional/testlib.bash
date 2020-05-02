#!/usr/bin/bash

top_srcdir="$(cd "$(dirname "${BASH_SOURCE[0]}")"/../../ && pwd)"
export top_srcdir
export FUNCTIONAL_TEST_DIR="${top_srcdir}/tests/functional"
export CLR_INSTALLER_EXE="${top_srcdir}/.gopath/bin/clr-installer"
export SCRIPTS="${top_srcdir}/scripts"
export TESTSCRIPTS="${top_srcdir}/tests"
export TESTWORKINGDIRBASENAME="${top_srcdir}/test"

# global variables

setup() {
    global_setup
    test_setup
}

teardown() {
    test_teardown
    global_teardown
}


global_setup() {
	# if global_setup is not defined it will default to this one
	echo "No global setup was defined"
}

global_teardown() {
	# if global_teardown is not defined it will default to this one
	echo "No global teardown was defined"
}

# Default test_setup
test_setup() {
	echo "No test_setup was defined"
}

# Default test_teardown
test_teardown() {
	echo "No test_teardown was defined"
}

assert_status_is_not() { # assertion

	local not_expected_status=$1

	if [ -z "$status" ]; then
		echo "The \$status environment variable is empty."
		echo "Please make sure this assertion is used inside a BATS test after a 'run' command."
		return 1
	fi

	if [ "$status" -eq "$not_expected_status" ]; then
		print_assert_failure "Status expected to be different than: $not_expected_status\\nActual status: $status"
		return 1
	else
		# if the assertion was successful show the output only if the user
		# runs the test with the -t flag
		echo -e "\\nCommand output:" >&3
		echo "------------------------------------------------------------------" >&3
		echo "$output" >&3
		echo -e "------------------------------------------------------------------\\n" >&3
	fi

}

assert_status_is() { # assertion

	local expected_status=$1

	if [ -z "$status" ]; then
		echo "The \$status environment variable is empty."
		echo "Please make sure this assertion is used inside a BATS test after a 'run' command."
		return 1
	fi

	if [ ! "$status" -eq "$expected_status" ]; then
		echo "Expected status: $expected_status\\nActual status: $status"
		return 1
	else
		# if the assertion was successful show the output only if the user
		# runs the test with the -t flag
		echo -e "\\nCommand output:" >&3
		echo "------------------------------------------------------------------" >&3
		echo "$output" >&3
		echo -e "------------------------------------------------------------------\\n" >&3
	fi

}

# Parameters: Raid device to be checked
# usage: wait_for_raid_to_be_active <raid_dev_name>
wait_for_raid_to_be_active() {
    let timeoutval=`expr 3 \* 60`
    let interval=1
    local raiddev=$1

    if [ -z "$raiddev" ]; then
        echo "RAID device name cant be empty" >&3
        exit 1
    fi

    if [ ! $mdadm --detail $raiddev ]; then
        echo "Invalid RAID device" >&3
        exit 1
    fi

    while [ "$timeoutval" -gt "0" ]
    do
        if [[ $(mdadm --detail $raiddev | grep -e "State : clean $") ]]; then
            break
        else
            echo "Waiting for RAID to be active" >&3
            sleep "$interval"
            timeoutval=`expr "$timeoutval" - "$interval"`
        fi
    done

    if [ "$timeoutval" -lt "0" ]; then
        echo "The wait timed out" >&3
        exit 1
    fi

}

create_testworking_dir() {
	local testdirname=$(echo $$)

	if [ -z "$testdirname" ]; then
        echo "Test dir name cant be empty" >&3
        exit 1
    fi

	local TESTDIRPATH=${TESTWORKINGDIRBASENAME}_${testdirname}

	if [ ! -d "$TESTDIRPATH" ]; then
		mkdir -p "$TESTDIRPATH"
		export TESTWORKINGDIR=${TESTDIRPATH}
	else
		echo "Fail to create test directory: ${TESTDIRPATH}" >&3
		exit 1
	fi
}

clean_testworking_dir() {

	if [ -z "${TESTWORKINGDIR}" ]; then
        echo "TEST Directory not set" >&3
        exit 1
    fi

	if [ -d "$TESTWORKINGDIR" ]; then
		rm -r "$TESTWORKINGDIR"
		unset ${TESTWORKINGDIR}
	else
		echo "Fail to delete test directory" >&3
		exit 1
	fi

}

