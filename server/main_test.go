// Copyright (c) 2026, NuDanOS contributors.
//
// SPDX-License-Identifier: LGPL-2.1-only

package server_test

import (
	"os"
	"testing"

	"github.com/danos/configd/server"
)

// Every commit saves the running config: through a temporary file in
// tmpDir (created in production by configd's tmpfiles.d entry) to
// configDir/config.boot, ending it with the version footer printed by
// vyatta_current_conf_ver.pl. None of these exists in a build environment,
// and without the footer a second save refuses to overwrite the first.
func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "configd-server-test")
	if err != nil {
		panic(err)
	}
	server.SetTmpDir(dir)
	server.SetConfigDir(dir)
	server.SetCurrentConfigVersion(func() string {
		return "/* === vyatta-config-version: \"test@1\" === */\n"
	})
	code := m.Run()
	os.RemoveAll(dir)
	os.Exit(code)
}
