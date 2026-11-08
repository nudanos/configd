// SPDX-License-Identifier: LGPL-2.1-only
package server

import (
	"github.com/danos/config/auth"
	"github.com/danos/configd"
	"github.com/danos/configd/rpc"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"syscall"
	"testing"
)

func TestCallArgumentValidation(t *testing.T) {
	fn := func(_ *Disp, db rpc.DB) (int, error) { return int(db), nil }
	conn := &SrvConn{srv: &Srv{m: map[string]reflect.Method{"Test": {Func: reflect.ValueOf(fn)}}}}
	disp := &Disp{ctx: &configd.Context{Auth: auth.TestAutherAllowAll()}}
	for _, value := range []interface{}{nil, "wrong", float64(99), float64(1.5)} {
		_, err := conn.Call(disp, "Test", []interface{}{value})
		if _, ok := err.(*rpc.ArgErr); !ok {
			t.Fatalf("argument %v: expected ArgErr, got %v", value, err)
		}
	}
	if _, err := conn.Call(disp, "Test", nil); err == nil {
		t.Fatal("missing argument accepted")
	}
	result, err := conn.Call(disp, "Test", []interface{}{float64(2)})
	if err != nil || result != 2 {
		t.Fatalf("numeric conversion: result %v error %v", result, err)
	}
}

func TestPeerCredentialsRetainUIDWhenLoginUIDUnset(t *testing.T) {
	if os.Getenv("CONFIGD_PEER_TEST_SOCKET") != "" {
		c, err := net.Dial("unix", os.Getenv("CONFIGD_PEER_TEST_SOCKET"))
		if err != nil {
			t.Fatal(err)
		}
		io.Copy(io.Discard, c)
		c.Close()
		return
	}
	if os.Geteuid() != 0 {
		t.Skip("requires root to launch unprivileged peer")
	}
	if _, err := getLoginUid(int32(os.Getpid())); !IsLoginPidError(err) {
		t.Skip("requires unset audit loginuid")
	}
	dir, err := os.MkdirTemp("", "configd-peer-")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)
	if err := os.Chmod(dir, 0777); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "socket")
	listener, err := net.ListenUnix("unix", &net.UnixAddr{Name: path, Net: "unix"})
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	if err := os.Chmod(path, 0777); err != nil {
		t.Fatal(err)
	}
	binary, err := os.ReadFile(os.Args[0])
	if err != nil {
		t.Fatal(err)
	}
	childPath := filepath.Join(dir, "peer-test")
	if err := os.WriteFile(childPath, binary, 0755); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(childPath, "-test.run=^TestPeerCredentialsRetainUIDWhenLoginUIDUnset$")
	cmd.Env = append(os.Environ(), "CONFIGD_PEER_TEST_SOCKET="+path)
	cmd.SysProcAttr = &syscall.SysProcAttr{Credential: &syscall.Credential{Uid: 65534, Gid: 65534}}
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	c, err := listener.AcceptUnix()
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	cred, err := (&SrvConn{UnixConn: c, srv: &Srv{}}).getCreds()
	if !IsLoginPidError(err) {
		t.Fatalf("expected unset loginuid, got %v", err)
	}
	if cred.Uid != 65534 {
		t.Fatalf("peer UID elevated: got %d, want 65534", cred.Uid)
	}
	c.Close()
	if err := cmd.Wait(); err != nil {
		t.Fatal(err)
	}
}

func TestResolvePeerUID(t *testing.T) {
	got, err := resolvePeerUID(1000, 0, 99, newLoginPidError(1))
	if got != 1000 || !IsLoginPidError(err) {
		t.Fatalf("unset login UID: got %d, %v", got, err)
	}
	for _, tc := range []struct {
		peer, login, daemon uint32
		loginErr            error
		want                uint32
		wantErr             bool
	}{

		{1000, 1000, 99, nil, 1000, false},
		{0, 1000, 99, nil, 1000, false},
		{99, 1000, 99, nil, 1000, false},
		{1000, 0, 99, nil, 1000, false},
		{1000, 99, 99, nil, 1000, false},
	} {
		got, err := resolvePeerUID(tc.peer, tc.login, tc.daemon, tc.loginErr)
		if (err != nil) != tc.wantErr || (!tc.wantErr && got != tc.want) {
			t.Errorf("peer=%d login=%d daemon=%d: got %d, %v", tc.peer, tc.login, tc.daemon, got, err)
		}
	}
}

func TestCallRejectsInvalidDatabaseForGet(t *testing.T) {
	method, ok := reflect.TypeOf((*Disp)(nil)).MethodByName("Get")
	if !ok {
		t.Fatal("Get method missing")
	}
	conn := &SrvConn{srv: &Srv{m: map[string]reflect.Method{"Get": method}}}
	disp := &Disp{ctx: &configd.Context{Auth: auth.TestAutherAllowAll()}}
	for _, value := range []interface{}{float64(99), float64(1.5)} {
		_, err := conn.Call(disp, "Get", []interface{}{value, "", ""})
		if _, ok := err.(*rpc.ArgErr); !ok {
			t.Fatalf("database %v: expected ArgErr, got %v", value, err)
		}
	}
}
