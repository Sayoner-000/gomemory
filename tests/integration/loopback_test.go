package main

import (
	"errors"
	"net"
	"runtime"
	"syscall"
	"testing"
	"time"
)

func loopbackPermissionDenied(err error) bool {
	return errors.Is(err, syscall.EPERM) || errors.Is(err, syscall.EACCES) ||
		(runtime.GOOS == "windows" && errors.Is(err, syscall.Errno(10013))) // WSAEACCES
}

// skipIfNoLoopback comprueba bind Y connect: algunos sandboxes solo deniegan
// el segundo. No oculta errores de red ajenos a una restricción de permisos.
func skipIfNoLoopback(t *testing.T) {
	t.Helper()
	l, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		if loopbackPermissionDenied(err) {
			t.Skipf("loopback TCP no permitido (listen): %v", err)
		}
		t.Fatalf("comprobar loopback TCP (listen): %v", err)
	}
	defer func() { _ = l.Close() }()
	c, err := net.DialTimeout("tcp4", l.Addr().String(), time.Second)
	if err != nil {
		if loopbackPermissionDenied(err) {
			t.Skipf("loopback TCP no permitido (connect): %v", err)
		}
		t.Fatalf("comprobar loopback TCP (connect): %v", err)
	}
	_ = c.Close()
}

func TestLoopbackPermissionDenied(t *testing.T) {
	for _, err := range []error{syscall.EPERM, &net.OpError{Op: "dial", Err: syscall.EACCES}} {
		if !loopbackPermissionDenied(err) {
			t.Fatalf("no reconoce restricción: %v", err)
		}
	}
	for _, err := range []error{nil, syscall.ECONNREFUSED, errors.New("unexpected error")} {
		if loopbackPermissionDenied(err) {
			t.Fatalf("oculta un error ajeno a permisos: %v", err)
		}
	}
}
