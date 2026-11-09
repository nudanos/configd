#!/bin/sh
# Run as root in a disposable Debian systemd VM without configd installed.
set -eu

postinst=$(CDPATH= cd -- "$(dirname -- "$0")/../debian" && pwd)/configd.postinst
test "$(id -u)" = 0
test -d /run/systemd/system
unit=/run/systemd/system/configd.service
test ! -e "$unit" && test ! -L "$unit" || {
    echo 'Refusing to replace an existing configd.service file' >&2
    exit 77
}
test "$(systemctl show --property=LoadState --value configd.service)" = not-found || {
    echo 'Refusing to replace an existing configd.service' >&2
    exit 77
}

work=$(mktemp -d /tmp/configd-upgrade-test.XXXXXX)
created=0
cleanup() {
    if [ "$created" = 1 ] && [ "$(readlink "$unit")" = "$work/configd.service" ]; then
        systemctl stop configd.service >/dev/null 2>&1 || true
        rm -f "$unit"
        systemctl daemon-reload
        systemctl reset-failed configd.service >/dev/null 2>&1 || true
    fi
    rm -rf -- "$work"
}
trap cleanup EXIT
trap 'exit 1' HUP INT TERM
mkdir -p "$work/bin" "$work/root/usr/sbin"
# Only account/capability setup is stubbed. Service operations use real systemd
# and Debian's installed deb-systemd-invoke, including its policy handling.
printf '#!/bin/sh\nexit 0\n' > "$work/bin/getent"
cp "$work/bin/getent" "$work/bin/setcap"
chmod 755 "$work/bin/getent" "$work/bin/setcap"
cat > "$work/configd.service" <<'UNIT'
[Service]
Type=exec
ExecStart=/bin/sleep infinity
UNIT
# Symlink creation refuses every existing path atomically, including dangling
# links and files that systemd has not loaded yet.
created=1
ln -s "$work/configd.service" "$unit"
systemctl daemon-reload

run_postinst() {
    env PATH="$work/bin:$PATH" DPKG_ROOT="$work/root" \
        RESTART_TEST_POLICY_LOG="$work/policy.log" sh "$postinst" "$@"
}
pid() { systemctl show --property=MainPID --value configd.service; }
fail() { echo "FAIL: $*" >&2; exit 1; }

systemctl start configd.service
before=$(pid)
run_postinst configure 2.15
test "$(pid)" != "$before" || fail 'upgrading an active daemon kept the old PID'
test "$(pid)" != 0 || fail 'upgrading stopped the daemon'
echo 'PASS: active upgrade replaces the daemon'

systemctl stop configd.service
run_postinst configure 2.16
test "$(systemctl show --property=ActiveState --value configd.service)" = inactive ||
    fail 'upgrading started a stopped daemon'
echo 'PASS: stopped daemon stays stopped'

systemctl start configd.service
before=$(pid)
run_postinst configure
test "$(pid)" = "$before" || fail 'fresh configuration restarted a running daemon'
echo 'PASS: fresh configuration does not restart'

cat > "$work/root/usr/sbin/policy-rc.d" <<'POLICY'
#!/bin/sh
printf '%s\n' "$*" >> "$RESTART_TEST_POLICY_LOG"
test "$2" != try-restart || exit 101
exit 0
POLICY
chmod 755 "$work/root/usr/sbin/policy-rc.d"
before=$(pid)
run_postinst configure 2.16
test "$(pid)" = "$before" || fail 'upgrade ignored policy-rc.d denial'
grep -qx 'configd.service try-restart' "$work/policy.log" ||
    fail 'upgrade did not consult policy for try-restart'
echo 'PASS: explicit policy denial is respected'
rm "$work/root/usr/sbin/policy-rc.d"

cat > "$work/configd.service" <<'UNIT'
[Service]
Type=oneshot
ExecStart=/bin/false
UNIT
systemctl daemon-reload
if run_postinst configure 2.16; then
    fail 'upgrade ignored a failed restart'
fi
echo 'PASS: restart failure is reported'
