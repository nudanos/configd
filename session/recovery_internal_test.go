// SPDX-License-Identifier: LGPL-2.1-only
package session

import (
	"github.com/danos/configd"
	"os"
	"path/filepath"
	"testing"
)

func TestWriteRunningPreservesPreviousFileWhenShowFails(t *testing.T) {
	path := filepath.Join(t.TempDir(), "running")
	if err := os.WriteFile(path, []byte("previous recovery data"), 0600); err != nil {
		t.Fatal(err)
	}
	terminated := make(chan struct{})
	close(terminated)
	mgr := &CommitMgr{effective: &Session{s: session{term: terminated}}}
	ctx := &configd.Context{Config: &configd.Config{Runfile: path}}
	if err := mgr.writeRunning(ctx); err == nil {
		t.Fatal("expected Show failure")
	}
	contents, err := os.ReadFile(path)
	if err != nil || string(contents) != "previous recovery data" {
		t.Fatalf("previous recovery data lost: %q, %v", contents, err)
	}
}
