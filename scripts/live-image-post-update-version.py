#!/usr/bin/python3

# c-basic-offset: 4; tab-width: 4; indent-tabs-mode: nil
# vi: set shiftwidth=4 tabstop=4 expandtab:
# :indentSize=4:tabSize=4:noTabs=true:

import os
import sys

def append_rootwait(path):
    """Add a delay to the installer kernel commandline"""
    entry_path = path + "/boot/loader/entries/"
    entry_file = os.listdir(entry_path)
    if len(entry_file) != 1:
        raise Exception("Unable to find specific entry file in {0}, "
                        "found {1} instead".format(entry_path, entry_file))
    file_full_path = entry_path + entry_file[0]
    with open(file_full_path, "r") as entry:
        entry_content = entry.readlines()
    options_line = entry_content[-1]
    if not options_line.startswith("options "):
        raise Exception("Last line of entry file is not the kernel "
                        "commandline options")
    # Account for newline at the end of the line
    options_line = options_line[:-1] + " rootwait\n"
    entry_content[-1] = options_line
    os.unlink(file_full_path)
    with open(file_full_path, "w") as entry:
        entry.writelines(entry_content)


if __name__ == '__main__':
    if len(sys.argv) != 2:
        sys.exit(-1)

    try:
        append_rootwait(sys.argv[1])
    except Exception as exep:
        print(exep)
        sys.exit(-1)
    sys.exit(0)

# Editor modelines  -  https://www.wireshark.org/tools/modelines.html
#
# Local variables:
# c-basic-offset: 4
# tab-width: 4
# indent-tabs-mode: nil
# End:
#
# vi: set shiftwidth=4 tabstop=4 expandtab:
# :indentSize=4:tabSize=4:noTabs=true:
#
