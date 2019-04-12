package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"sort"
	"strings"

	"github.com/Kochava/envi"
	"golang.org/x/sync/errgroup"
)

func main() {
	var (
		ctx  = context.Background()
		prog = Prog{
			stderr: os.Stderr,
			stdout: os.Stdout,
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

	stderr io.Writer
	stdout io.Writer
}

func (p *Prog) init(flags *flag.FlagSet, argv []string) (err error) {
	p.flags = flags
	p.conf = DefaultConfig()

	if err := envi.Getenv(p.conf, "GRIBBLE_"); err != nil && !envi.IsNoValue(err) {
		return err
	}

	flags.Usage = p.usage
	bindConfigFlags(flags, p.conf)

	return p.flags.Parse(argv)
}

func (p Prog) Run(ctx context.Context, flags *flag.FlagSet, argv ...string) (code int) {
	if err := p.init(flags, argv); err == flag.ErrHelp {
		return 2
	} else if err != nil {
		return 1
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	ctx = cancelOnSignal(ctx, shutdownSignals()...)

	// TODO: Configurable databases, probably, once the interface is better-defined.
	db, err := NewDatabase(ctx, p.conf)
	if err != nil {
		log.Printf("Unable to open database: %v", err)
		return 1
	}
	defer db.Close()

	p.db = db
	if err := db.Migrate(ctx); err != nil {
		log.Printf("Migrations failed: %v", err)
		return 1
	}

	listener, err := p.listen()
	if err != nil {
		log.Printf("Error listening on configured address (%v): %v", p.conf.Listen, err)
	}
	defer listener.Close() // will double-close on successful runs
	log.Printf("Listening on address: %v", listener.Addr())

	wg, ctx := errgroup.WithContext(ctx)
	defer func() {
		if err := wg.Wait(); err != nil && err != context.Canceled {
			log.Printf("Fatal error: %v", err)
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
	p.server, err = NewServer(nil, p.db) // TODO: Configure server
	if err != nil {
		return err
	}

	log.Printf("Server token created: %+q", p.server.Token())

	sv := &http.Server{
		Handler: AccessLog(p.server),
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
		log.Printf("Server shutdown: %v", addr)
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
`)
}

func bindConfigFlags(f *flag.FlagSet, conf *Config) {
	f.Var(NewTextFlag(&conf.DB), "backend", "Database `backend`")
	f.StringVar(&conf.SQLiteFile, "sqlite-file", conf.SQLiteFile, "SQLite database file")
	f.IntVar(&conf.SQLitePoolSize, "sqlite-pool-size", conf.SQLitePoolSize, "SQLite pool size")
	f.Var(NewTextFlag(conf.Listen), "http-listen-addr", "Listen `address`")
	f.DurationVar(&conf.GracePeriod, "http-grace-period", conf.GracePeriod, "Shutdown grace period")
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
