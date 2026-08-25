//go:build !windows

package auth

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/creack/pty"
	"golang.org/x/sys/unix"
	"golang.org/x/term"
)

func TestReadTokenTerminalIsHiddenEditableAndRestored(t *testing.T) {
	master, slave, err := pty.Open()
	if err != nil {
		t.Fatalf("open PTY: %v", err)
	}
	t.Cleanup(func() {
		_ = master.Close()
		_ = slave.Close()
	})
	before, err := term.GetState(int(slave.Fd()))
	if err != nil {
		t.Fatalf("read initial terminal state: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	type result struct {
		credential Credential
		err        error
	}
	completed := make(chan result, 1)
	go func() {
		credential, readErr := ReadToken(ctx, slave)
		completed <- result{credential: credential, err: readErr}
	}()

	deadline := time.Now().Add(time.Second)
	for {
		current, stateErr := term.GetState(int(slave.Fd()))
		if stateErr != nil {
			t.Fatalf("read terminal state: %v", stateErr)
		}
		if !reflect.DeepEqual(before, current) {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("terminal did not enter hidden raw mode")
		}
		time.Sleep(time.Millisecond)
	}
	if _, err := master.Write([]byte("\b\x7f\b\x7fabcx\bdey\x7f\r")); err != nil {
		t.Fatalf("write PTY input: %v", err)
	}
	read := <-completed
	if read.err != nil {
		t.Fatalf("ReadToken() error = %v", read.err)
	}
	if read.credential.Token != "abcde" {
		t.Fatalf("token = %q, want %q", read.credential.Token, "abcde")
	}
	after, err := term.GetState(int(slave.Fd()))
	if err != nil {
		t.Fatalf("read restored terminal state: %v", err)
	}
	if !reflect.DeepEqual(before, after) {
		t.Fatal("terminal state was not restored")
	}
	masterFD := int(master.Fd())
	if err := unix.SetNonblock(masterFD, true); err != nil {
		t.Fatalf("make PTY master non-blocking: %v", err)
	}
	t.Cleanup(func() { _ = unix.SetNonblock(masterFD, false) })
	buffer := make([]byte, 64)
	count, err := unix.Read(masterFD, buffer)
	if count > 0 {
		t.Fatalf("terminal echoed token input: %q", buffer[:count])
	}
	if err == nil || (!errors.Is(err, unix.EAGAIN) && !errors.Is(err, unix.EWOULDBLOCK)) {
		t.Fatalf("PTY read error = %v, want EAGAIN/EWOULDBLOCK", err)
	}
}
