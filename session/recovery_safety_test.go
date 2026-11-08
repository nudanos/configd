// SPDX-License-Identifier: LGPL-2.1-only
package session_test

import (
	. "github.com/danos/configd/session/sessiontest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCommitReportsRecoveryFileFailure(t *testing.T) {
	srv, sess := TstStartup(t, `container test { leaf value { type string; } }`, "")
	defer sess.Kill()
	// A directory cannot be replaced by the running-configuration file.
	srv.Ctx.Config.Runfile = t.TempDir()
	ValidateSet(t, sess, srv.Ctx, []string{"test", "value", "new"}, false)
	_, errs, ok := sess.Commit(srv.Ctx, "", false)
	if !ok {
		t.Fatal("already applied commit must retain success status")
	}
	if len(errs) == 0 {
		t.Fatal("recovery persistence failure was not reported")
	}
}

func TestCommitPersistsRecoveryFile(t *testing.T) {
	srv, sess := TstStartup(t, `container test { leaf value { type string; } }`, "")
	defer sess.Kill()
	path := filepath.Join(t.TempDir(), "running")
	srv.Ctx.Config.Runfile = path
	ValidateSet(t, sess, srv.Ctx, []string{"test", "value", "new"}, false)
	_, errs, ok := sess.Commit(srv.Ctx, "", false)
	if !ok || len(errs) != 0 {
		t.Fatalf("commit: ok=%v errors=%v", ok, errs)
	}
	contents, err := os.ReadFile(path)
	if err != nil || !strings.Contains(string(contents), "new") {
		t.Fatalf("recovery file: %q, %v", contents, err)
	}
	info, err := os.Stat(path)
	if err != nil || info.Mode().Perm() != 0600 {
		t.Fatalf("recovery file mode: %v, %v", info, err)
	}
}
