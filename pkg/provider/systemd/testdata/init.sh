#!/bin/sh
# Container init for the systemd integration tests. Runs as root and brings
# up a systemd user manager for the host's uid (setpriv needs no passwd
# entry), so the provider connecting as that same uid can manage units
# without polkit. The bus socket and unit files live in the bind-mounted
# TEST_UNITS dir, visible to the host at the same paths.
set -e

C=$(cut -d/ -f2- /proc/1/cgroup)
# The integration container is allowed to delegate only its own cgroup subtree.
# An empty or root path would make the recursive chown affect the host mount.
case "$C" in
	""|"/")
		echo "refusing to change ownership of an unsafe cgroup path: $C" >&2
		exit 1
		;;
esac
chown -R "$TEST_UID" "/sys/fs/cgroup/$C"
mkdir -p /run/systemd/system

setpriv --reuid "$TEST_UID" --regid "$TEST_UID" --clear-groups env XDG_RUNTIME_DIR="$TEST_UNITS" \
	/usr/bin/dbus-daemon --session --nofork --address=unix:path="$TEST_UNITS/bus" &
while [ ! -S "$TEST_UNITS/bus" ]; do
	sleep 0.05
done
setpriv --reuid "$TEST_UID" --regid "$TEST_UID" --clear-groups env XDG_RUNTIME_DIR="$TEST_UNITS" \
	XDG_CONFIG_HOME="$TEST_UNITS/.config" HOME="$TEST_UNITS" /usr/bin/systemd --user &
wait
