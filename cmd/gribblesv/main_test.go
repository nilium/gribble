package main

import (
	"bytes"
	"context"
	"flag"
	"io"
	"testing"
	"time"
)

type prefixLogger struct {
	testing.TB
	Prefix func() string
}

var prefixNewline = []byte{'\n'}

func (l prefixLogger) Write(p []byte) (n int, err error) {
	n = len(p)
	prefix := l.Prefix()
	p = bytes.TrimSuffix(p, prefixNewline)
	lines := bytes.Split(p, prefixNewline)
	for _, line := range lines {
		l.TB.Logf("%s%s", prefix, line)
	}
	return n, nil
}

type testProg struct {
	*Prog
	Start time.Time
	Flags *flag.FlagSet

	// Buffers to collect stdout/stderr output
	Stdout bytes.Buffer
	Stderr bytes.Buffer
}

func (p *testProg) deltaPrinter(name string) func() string {
	return func() string {
		d := time.Since(p.Start).String()
		return name + "\t" + d + "\t"
	}
}

func (p *testProg) Test(ctx context.Context, argv ...string) int {
	p.Start = time.Now()
	return p.Run(ctx, p.Flags, argv...)
}

func newProgram(tb testing.TB) (context.Context, *testProg) {
	ctx := context.Background()
	flags := flag.NewFlagSet(tb.Name(), flag.ContinueOnError)
	tprog := &testProg{
		Flags: flags,
	}
	prog := &Prog{
		stderr: io.MultiWriter(
			&tprog.Stderr,
			prefixLogger{tb, tprog.deltaPrinter("STDERR")},
		),
		stdout: io.MultiWriter(
			&tprog.Stdout,
			prefixLogger{tb, tprog.deltaPrinter("STDOUT")},
		),
	}
	tprog.Prog = prog
	return ctx, tprog
}

func TestCLIUsage(t *testing.T) {
	t.Run("ShortFlag", func(t *testing.T) {
		ctx, prog := newProgram(t)
		if got := prog.Test(ctx, "-h"); got != 2 {
			t.Errorf("-h = %d; want %d", got, 2)
		}
	})
	t.Run("LongFlag", func(t *testing.T) {
		ctx, prog := newProgram(t)
		if got := prog.Test(ctx, "--help"); got != 2 {
			t.Errorf("--help = %d; want %d", got, 2)
		}
	})
	t.Run("InvalidFlag", func(t *testing.T) {
		ctx, prog := newProgram(t)
		if got := prog.Test(ctx, "--invalid-flag"); got != 1 {
			t.Errorf("--invalid-flag = %d; want %d", got, 1)
		}
	})
}
