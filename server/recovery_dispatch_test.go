// SPDX-License-Identifier: LGPL-2.1-only
package server

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/danos/configd/session/sessiontest"
)

func TestConfirmedCommitSchedulesRollbackAfterRecoveryWarning(t *testing.T) {
	srv, sess := sessiontest.TstStartup(t, `container test { leaf value { type string; } }`, "")
	defer sess.Kill()
	dir := t.TempDir()
	oldConfig, oldTmp, oldCmd := configDir, tmpDir, configMgmtCmd
	configDir, tmpDir = dir, dir
	t.Cleanup(func() { configDir, tmpDir, configMgmtCmd = oldConfig, oldTmp, oldCmd })
	marker := filepath.Join(dir, "scheduled")
	configMgmtCmd = filepath.Join(dir, "config-mgmt")
	if err := os.WriteFile(configMgmtCmd, []byte("#!/bin/sh\nprintf '%s' \"$*\" > '"+marker+"'\n"), 0700); err != nil {
		t.Fatal(err)
	}
	srv.Ctx.Config.Runfile = t.TempDir()
	d := NewDispatcher(srv.Smgr, srv.Cmgr, srv.Ms, srv.MsFull, srv.Ctx)
	if _, err := d.SessionSetup("recovery"); err != nil {
		t.Fatal(err)
	}
	if _, err := d.Set("recovery", "test/value/new"); err != nil {
		t.Fatal(err)
	}
	output, err := d.confirmedCommitInternal("recovery", "", false, 0, &commitInfo{confirmed: true, timeout: 60}, false)
	if err != nil {
		t.Fatalf("commit: %v", err)
	}
	if !strings.Contains(output, "persist running configuration") {
		t.Fatalf("missing recovery warning: %q", output)
	}
	got, err := os.ReadFile(marker)
	if err != nil || !strings.Contains(string(got), "--action=confirmed-commit") {
		t.Fatalf("rollback not scheduled: %q, %v", got, err)
	}
}

func TestRunningConfigurationRestoredOnRestart(t *testing.T) {
	srv, sess := sessiontest.TstStartup(t, `container test { leaf value { type string; } }`, "")
	defer sess.Kill()
	srv.Ctx.Config.Runfile = filepath.Join(t.TempDir(), "running")
	sessiontest.ValidateSet(t, sess, srv.Ctx, []string{"test", "value", "new"}, false)
	if _, errs, ok := sess.Commit(srv.Ctx, "", false); !ok || len(errs) != 0 {
		t.Fatalf("commit: ok=%v errors=%v", ok, errs)
	}
	restored := loadRunning(srv.Ctx.Config, srv.Ms)
	if restored.Child("test").Child("value").Child("new") == nil {
		t.Fatal("startup did not restore committed value")
	}
}

func TestConfirmedCommitReportsRecoveryAndSchedulingFailures(t *testing.T) {
	srv, sess := sessiontest.TstStartup(t, `container test { leaf value { type string; } }`, "")
	defer sess.Kill()
	dir := t.TempDir()
	oldConfig, oldTmp, oldCmd := configDir, tmpDir, configMgmtCmd
	configDir, tmpDir = dir, dir
	t.Cleanup(func() { configDir, tmpDir, configMgmtCmd = oldConfig, oldTmp, oldCmd })
	configMgmtCmd = filepath.Join(dir, "config-mgmt")
	if err := os.WriteFile(configMgmtCmd, []byte("#!/bin/sh\necho scheduler-failed\nexit 1\n"), 0700); err != nil {
		t.Fatal(err)
	}
	srv.Ctx.Config.Runfile = t.TempDir()
	d := NewDispatcher(srv.Smgr, srv.Cmgr, srv.Ms, srv.MsFull, srv.Ctx)
	if _, err := d.SessionSetup("recovery"); err != nil {
		t.Fatal(err)
	}
	if _, err := d.Set("recovery", "test/value/new"); err != nil {
		t.Fatal(err)
	}
	_, err := d.confirmedCommitInternal("recovery", "", false, 0, &commitInfo{confirmed: true, timeout: 60}, false)
	if err == nil || !strings.Contains(err.Error(), "persist running configuration") || !strings.Contains(err.Error(), "scheduler-failed") {
		t.Fatalf("expected both failures, got %v", err)
	}
}
