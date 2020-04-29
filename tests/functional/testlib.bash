#!/usr/bin/bash

# global variables
top_srcdir="$(cd "$(dirname "${BASH_SOURCE[0]}")"/../../ && pwd)"
export top_srcdir
export FUNCTIONAL_TEST_DIR="$top_srcdir/tests/functional"
export CLR_INSTALLER_EXE="$top_srcdir/.gopath/bin/clr-installer"
export SCRIPTS="$top_srcdir/scripts"
export TESTSCRIPTS="$top_srcdir/tests"
export TESTWORKINGDIRECTORY="$top_srcdir/testworking"

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

create_testworking_dir(){
	mkdir -p "$TESTWORKINGDIRECTORY"
}

clean_testworking_dir(){
	rm -r "$TESTWORKINGDIRECTORY"
}

