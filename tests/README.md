`upgrade-restart.sh` exercises the source maintainer script with real systemd
and Debian's service-policy helper. Run it as root in a disposable Debian VM
without configd installed. It refuses an existing unit, creates a temporary
service under `/run`, and removes the fixture afterward. Only account creation
and file-capability setup are stubbed.

It checks active and stopped upgrades, fresh configuration, explicit
`policy-rc.d` denial, and restart failure. The appliance acceptance tests must
also verify the running executable and real expired-password renewal after an
APT upgrade; this service fixture does not replace those tests.
