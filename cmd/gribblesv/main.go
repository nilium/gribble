package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/signal"
	"sort"
	"strings"

	"github.com/Kochava/envi"
	"go.spiff.io/gribble/internal/proc"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"golang.org/x/sync/errgroup"
)

func main() {
	var (
		ctx  = context.Background()
		prog = Prog{
			stderr:           os.Stderr,
			stdout:           os.Stdout,
			setDefaultLogger: true,
		}
		argv = os.Args[1:]
		code = prog.Run(ctx, flag.CommandLine, argv...)
	)
	os.Exit(code)
}

type Prog struct {
	conf   *Config
	server *Server
	flags  *flag.FlagSet
	db     DB

	setDefaultLogger bool
	logLevel         zap.AtomicLevel
	logger           *zap.Logger

	stderr io.Writer
	stdout io.Writer
}

func (p *Prog) init(flags *flag.FlagSet, argv []string) (err error) {
	p.flags = flags
	p.conf = DefaultConfig()

	p.logLevel = zap.NewAtomicLevel()

	if err := envi.Getenv(p.conf, "GRIBBLE_"); err != nil && !envi.IsNoValue(err) {
		return err
	}

	flags.Usage = p.usage
	bindConfigFlags(flags, p.conf)

	if err := p.flags.Parse(argv); err != nil {
		return err
	}

	p.logLevel.SetLevel(p.conf.LogLevel)
	zapconf := zap.NewProductionConfig()
	if !p.conf.LogJSON {
		zapconf.Encoding = "console"
		zapconf.EncoderConfig.EncodeLevel = zapcore.CapitalLevelEncoder
	}
	zapconf.EncoderConfig.EncodeDuration = zapcore.SecondsDurationEncoder
	zapconf.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder

	logger, err := zapconf.Build()
	if err != nil {
		return err
	}

	p.logger = logger
	if p.setDefaultLogger {
		zap.ReplaceGlobals(p.logger)
	}

	return nil
}

func (p Prog) Run(ctx context.Context, flags *flag.FlagSet, argv ...string) (code int) {
	if err := p.init(flags, argv); err == flag.ErrHelp {
		return 2
	} else if err != nil {
		fmt.Fprintf(p.stderr, "Unable to initialize gribblesv: %v\n", err)
		return 1
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	ctx = cancelOnSignal(ctx, shutdownSignals()...)

	// TODO: Configurable databases, probably, once the interface is better-defined.
	db, err := NewDatabase(ctx, p.conf)
	if err != nil {
		proc.DPanic(ctx, "Unable to open database", zap.Error(err))
		return 1
	}
	defer db.Close()

	p.db = db
	if err := db.Migrate(ctx); err != nil {
		proc.DPanic(ctx, "Migrations failed", zap.Error(err))
		return 1
	}

	listener, err := p.listen()
	if err != nil {
		proc.DPanic(ctx, "Error listening on configured address", zap.Stringer("addr", p.conf.Listen), zap.Error(err))
		return 1
	}
	defer listener.Close() // will double-close on successful runs
	proc.Info(ctx, "Listening", zap.Stringer("addr", listener.Addr()))

	wg, ctx := errgroup.WithContext(ctx)
	defer func() {
		if err := wg.Wait(); err != nil && err != context.Canceled {
			proc.Error(ctx, "Fatal error encountered", zap.Error(err))
			code = 1
		}
	}()

	wg.Go(func() error { return p.serve(ctx, listener) })

	<-ctx.Done()
	cancel()

	return 0
}

func (p *Prog) listen() (net.Listener, error) {
	network, addr := p.conf.Listen.ListenStreamArgs()
	if addr == "" {
		return nil, fmt.Errorf("listen address is not valid: %v", p.conf.Listen)
	}
	return net.Listen(network, addr)
}

func (p *Prog) serve(ctx context.Context, listener net.Listener) (err error) {
	conf := &ServerConfig{
		GitHubToken: p.conf.GitHubToken,
	}
	p.server, err = NewServer(conf, p.db) // TODO: Configure server
	if err != nil {
		return err
	}

	proc.Info(ctx, "Server token created", zap.String("token", p.server.Token()))

	sv := &http.Server{
		Handler: AccessLog(p.server, p.logger, zap.InfoLevel),
	}
	addr := listener.Addr()

	go func() {
		<-ctx.Done()
		timeout, cancel := context.WithTimeout(context.Background(), p.conf.GracePeriod)
		defer cancel()
		if err := sv.Shutdown(timeout); err == context.DeadlineExceeded {
			_ = sv.Close()
		}
	}()

	err = sv.Serve(listener)
	if err == http.ErrServerClosed {
		proc.Info(ctx, "Server has shutdown", zap.Stringer("addr", addr))
		return nil
	}
	return err
}

func (p *Prog) usage() {
	var backendNames []string
	for k := range backends {
		backendNames = append(backendNames, k.String())
	}
	sort.Strings(backendNames)
	fmtBackendNames := strings.Join(backendNames, "\n      - ")

	fmt.Fprint(p.stderr, `Usage: gribblesv [options]

Options:
  -h, -help
    Print this usage text.
  -http-listen-addr SOCKADDR (default `, defaultListenAddr, `)
    Address the HTTP server binds on. May be a TCP address and port
    (1.2.3.4:80) or a path to a Unix domain socket.
  -http-grace-period DUR (default `, defaultGracePeriod, `)
    HTTP server shutdown grace period.
  -github-token
    The GitHub token to validate GitHub events with.
    If not given, GitHub events are not accepted.
    Can be set to DEV (uppercase) to allow all events without
    validation.
  -backend BACKEND (default: `, defaultBackendName, `)
    Database driver backend.
    May be one of the following:
      - `, fmtBackendNames, `

SQLite Backend:
  -sqlite-file FILE (default: `, defaultSQLiteFile, `)
    SQLite database file.
    (sqlite)
  -sqlite-pool-size SIZE (default: `, defaultSQLitePoolSize, `)
    SQLite backend connection pool size.
    (sqlite, sqlite-memory)

Logging:
  -log-level LEVEL (default: `, defaultLogLevel, `)
    The level of logging verbosity. May be one of:
      debug, info, warn, error, panic, fatal
  -log-json
    Write logs in JSON instead of a console-friendly format.
`)
}

func bindConfigFlags(f *flag.FlagSet, conf *Config) {
	f.Var(NewTextFlag(conf.Listen), "http-listen-addr", "Listen `address`")
	f.DurationVar(&conf.GracePeriod, "http-grace-period", conf.GracePeriod, "Shutdown grace period")
	f.StringVar(&conf.GitHubToken, "github-token", conf.GitHubToken, "GitHub token")

	f.Var(NewTextFlag(&conf.DB), "backend", "Database `backend`")
	f.StringVar(&conf.SQLiteFile, "sqlite-file", conf.SQLiteFile, "SQLite database file")
	f.IntVar(&conf.SQLitePoolSize, "sqlite-pool-size", conf.SQLitePoolSize, "SQLite pool size")

	f.Var(&conf.LogLevel, "log-level", "Logging level")
	f.BoolVar(&conf.LogJSON, "log-json", conf.LogJSON, "Write JSON logs")
}

func cancelOnSignal(ctx context.Context, signals ...os.Signal) context.Context {
	ctx, cancel := context.WithCancel(ctx)
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, signals...)
	finish := func() { signal.Stop(sig); cancel() }
	go func() {
		defer finish()
		select {
		case <-sig:
		case <-ctx.Done():
		}
	}()
	return ctx
}
