// Copyright (c) 2019-2020, AT&T Intellectual Property Inc. All rights reserved.
//
// SPDX-License-Identifier: LGPL-2.1-only

package server_test

import (
	"github.com/danos/configd/session/sessiontest"
	"github.com/danos/configd/startuplock"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/danos/config/auth"
	"github.com/danos/config/testutils"
	"github.com/danos/configd/server"
)

func TestConfigMgmtProductionCallerCmdSetPrivs(t *testing.T) {
	if !server.GetProductionCallerCmdSetPrivs() {
		t.Fatalf("callerCmdSetPrivs unexpectedly disabled")
	}
}

func TestConfigMgmtProductionTmpDir(t *testing.T) {
	if server.GetProductionTmpDir() != "/var/tmp/configd" {
		t.Fatalf("Unexpected tmpDir %v", server.GetProductionTmpDir())
	}
}

func TestLoadFromSuccessCommandAuthz(t *testing.T) {
	a := auth.TestAutherAllowAll()
	d := newTestDispatcherWithCustomAuth(
		t, a,
		loadOrMergeSchema, initConfig,
		false, /* not configd user, so our auther gets used! */
		false /* not in secrets group */)

	dispTestSetupSession(t, d, testSID)

	testConfig := testutils.Root(
		testutils.Leaf("testint", "8"))

	file, err := dispTestLoadOrMergeWriteConfigToFile(testConfig)
	if err != nil {
		t.Fatalf("%s", err.Error())
	}
	defer os.Remove(file)

	ok, err := d.LoadFrom(testSID, file, "")
	if err != nil {
		t.Fatalf("%s", err.Error())
	}
	if !ok {
		t.Fatalf("LoadFrom failed with no error returned")
	}

	assertCommandAaaNoSecrets(t, a, []string{"load", file})
}

func TestLoadFromRoutingInstanceCommandAuthz(t *testing.T) {
	a := auth.TestAutherAllowAll()
	d := newTestDispatcherWithCustomAuth(
		t, a,
		loadOrMergeSchema, initConfig,
		false, /* not configd user, so our auther gets used! */
		false /* not in secrets group */)

	dispTestSetupSession(t, d, testSID)

	// Right now we're just checking command authorization
	_, _ = d.LoadFrom(testSID, "scp://bar:baz@localhost/conf", "red")

	assertCommandAaaNoSecrets(t, a,
		[]string{"load", "routing-instance", "red", "scp://bar:**@localhost/conf"})
}

func TestSaveToCommandAuthz(t *testing.T) {
	a := auth.TestAutherAllowAll()
	d := newTestDispatcherWithCustomAuth(
		t, a,
		loadOrMergeSchema, initConfig,
		false, /* not configd user, so our auther gets used! */
		false /* not in secrets group */)

	server.SetCallerCmdSetPrivs(false)
	defer server.SetCallerCmdSetPrivs(server.GetProductionCallerCmdSetPrivs())

	dispTestSetupSession(t, d, testSID)

	file := os.TempDir() + "/saveto"

	// Right now we're mostly just checking command authorization
	success, err := d.SaveTo(file, "")
	if err != nil {
		t.Fatalf("Unexpected err: %s", err)
	}
	if !success {
		t.Fatalf("SaveTo failed unexpectedly")
	}

	// Cleanup the saved config
	defer os.Remove(file)

	assertCommandAaaNoSecrets(t, a, []string{"save", file})
}

func TestSaveToRoutingInstanceCommandAuthz(t *testing.T) {
	a := auth.TestAutherAllowAll()
	d := newTestDispatcherWithCustomAuth(
		t, a,
		loadOrMergeSchema, initConfig,
		false, /* not configd user, so our auther gets used! */
		false /* not in secrets group */)

	dispTestSetupSession(t, d, testSID)

	// Right now we're just checking command authorization
	_, _ = d.SaveTo("scp://bar:baz@localhost/conf", "red")

	assertCommandAaaNoSecrets(t, a,
		[]string{"save", "routing-instance", "red", "scp://bar:**@localhost/conf"})
}

func TestLocalSaveWaitsForStartupWriter(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "startup-lock")
	oldDir := server.GetStartupLockDir()
	server.SetStartupLockDir(dir)
	defer server.SetStartupLockDir(oldDir)
	held, err := startuplock.Acquire(dir, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	a := auth.TestAutherAllowAll()
	srv, sess := sessiontest.NewTestSpec(t).SetSingleSchema(loadOrMergeSchema).
		SetConfig(initConfig).SetAuther(a, false, false).Init()
	defer sess.Kill()
	d := server.NewDispatcher(srv.Smgr, srv.Cmgr, srv.Ms, srv.MsFull, srv.Ctx)
	server.SetCallerCmdSetPrivs(false)
	defer server.SetCallerCmdSetPrivs(server.GetProductionCallerCmdSetPrivs())
	dispTestSetupSession(t, d, testSID)
	done := make(chan error, 1)
	dest := filepath.Join(t.TempDir(), "saved")
	go func() { _, err := d.SaveTo(dest, ""); done <- err }()
	select {
	case err := <-done:
		held.Close()
		t.Fatalf("save passed a held startup lock: %v", err)
	case <-time.After(250 * time.Millisecond):
	}
	// The lower-level session commit used by this fixture does not perform
	// the dispatcher's automatic Save. Change RUNNING here so the waiting
	// save must snapshot only afterwards.
	sessiontest.ValidateSet(t, sess, srv.Ctx, []string{"teststring", "fresh-value"}, false)
	if _, errs, ok := sess.Commit(srv.Ctx, "", false); !ok || len(errs) != 0 {
		t.Fatalf("runtime commit: ok=%v errors=%v", ok, errs)
	}
	held.Close()
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("save did not resume after lock release")
	}
	content, err := os.ReadFile(dest)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(content), "fresh-value") || strings.Contains(string(content), "stuff") {
		t.Fatalf("save used a stale running snapshot: %s", content)
	}
}

func TestLocalSaveWaitsBeforeValidatingDestination(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "startup-lock")
	oldDir := server.GetStartupLockDir()
	server.SetStartupLockDir(dir)
	defer server.SetStartupLockDir(oldDir)
	held, err := startuplock.Acquire(dir, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer held.Close()
	a := auth.TestAutherAllowAll()
	d := newTestDispatcherWithCustomAuth(t, a, loadOrMergeSchema, initConfig, false, false)
	server.SetCallerCmdSetPrivs(false)
	defer server.SetCallerCmdSetPrivs(server.GetProductionCallerCmdSetPrivs())
	dest := filepath.Join(t.TempDir(), "saved")
	valid := []byte("teststring previous\n/* === vyatta-config-version: \"test@1\" === */\n")
	if err := os.WriteFile(dest, valid, 0600); err != nil {
		t.Fatal(err)
	}
	// Emulate a cooperating writer's brief truncate/copy phase under the lock.
	if err := os.WriteFile(dest, nil, 0600); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { _, err := d.SaveTo(dest, ""); done <- err }()
	select {
	case err := <-done:
		t.Fatalf("save validated a destination while locked: %v", err)
	case <-time.After(250 * time.Millisecond):
	}
	if err := os.WriteFile(dest, valid, 0600); err != nil {
		t.Fatal(err)
	}
	if err := held.Close(); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("save after writer completed: %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("save did not resume after lock release")
	}
	got, err := os.ReadFile(dest)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(got), "stuff") || strings.Contains(string(got), "previous") {
		t.Fatalf("save did not write the current snapshot: %s", got)
	}
}
