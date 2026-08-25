package auth

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"unicode/utf8"

	"golang.org/x/term"
)

type fileDescriptor interface {
	Fd() uintptr
}

// ReadToken reads one bounded token. Terminal input is read without echo, and
// cancellation closes a blocking input when possible.
func ReadToken(ctx context.Context, input io.Reader) (Credential, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return Credential{}, err
	}
	if input == nil {
		return Credential{}, fmt.Errorf("%w: no input", ErrInvalidToken)
	}
	var terminalState *term.State
	if source, ok := input.(fileDescriptor); ok && term.IsTerminal(int(source.Fd())) {
		state, err := term.MakeRaw(int(source.Fd()))
		if err != nil {
			return Credential{}, fmt.Errorf("hide token input: %w", err)
		}
		terminalState = state
	}
	restore := func() {
		if terminalState == nil {
			return
		}
		if source, ok := input.(fileDescriptor); ok {
			_ = term.Restore(int(source.Fd()), terminalState)
		}
		terminalState = nil
	}
	defer restore()

	type result struct {
		raw []byte
		err error
	}
	completed := make(chan result, 1)
	go func() {
		raw, err := readBoundedLine(input, terminalState != nil)
		completed <- result{raw: raw, err: err}
	}()
	select {
	case <-ctx.Done():
		restore()
		if closer, ok := input.(io.Closer); ok {
			_ = closer.Close()
		}
		return Credential{}, ctx.Err()
	case read := <-completed:
		if read.err != nil {
			return Credential{}, fmt.Errorf("read token: %w", read.err)
		}
		credential := Credential{Token: string(read.raw)}
		if err := credential.Validate(); err != nil {
			return Credential{}, err
		}
		return credential, nil
	}
}

func readBoundedLine(input io.Reader, terminal bool) ([]byte, error) {
	reader := bufio.NewReaderSize(input, 256)
	raw := make([]byte, 0, 256)
	for {
		value, err := reader.ReadByte()
		if err != nil {
			if errors.Is(err, io.EOF) {
				return raw, nil
			}
			return nil, err
		}
		switch value {
		case '\n', '\r', 0x04:
			return raw, nil
		case 0x03:
			return nil, context.Canceled
		case 0x08, 0x7f:
			if terminal {
				if len(raw) > 0 {
					_, size := utf8.DecodeLastRune(raw)
					raw = raw[:len(raw)-size]
				}
				continue
			}
		}
		if len(raw) == MaxTokenBytes {
			return nil, fmt.Errorf("%w: token exceeds %d bytes", ErrInvalidToken, MaxTokenBytes)
		}
		raw = append(raw, value)
	}
}
