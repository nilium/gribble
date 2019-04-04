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
	"time"

	"github.com/julienschmidt/httprouter"
	"go.spiff.io/gribble/internal/sqlite"
	"golang.org/x/sync/errgroup"
)

const (
	defaultListenAddr  = "127.0.0.1:4077"
	defaultGracePeriod = time.Second * 30
)

type DB interface {
	Migrate(context.Context) error
}

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

func (p *Prog) init(flags *flag.FlagSet, argv []string) error {
	p.flags = flags
	p.conf = &Config{
		Listen: newSockAddr(defaultListenAddr),
	}

	p.server = &Server{}

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
	db, err := sqlite.NewFileDB(ctx, "gribble.db", 8)
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

func (p *Prog) serve(ctx context.Context, listener net.Listener) error {
	mux := httprouter.New()
	mux.POST("/_gitlab/api/v4/runners", HandleJSON(p.server.RegisterRunner))
	mux.POST("/_gitlab/api/v4/jobs/request", HandleJSON(p.server.RequestJob))
	mux.PATCH("/_gitlab/api/v4/jobs/:id/trace", HandleJSON(p.server.PatchTrace))
	mux.PUT("/_gitlab/api/v4/jobs/:id", HandleJSON(p.server.UpdateJob))

	logger := AccessLog(mux)

	sv := &http.Server{
		Handler: logger,
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

	err := sv.Serve(listener)
	if err == http.ErrServerClosed {
		log.Printf("Server shutdown: %v", addr)
		return nil
	}
	return err
}

func (p *Prog) usage() {
	fmt.Fprint(p.stderr, `Usage: gribblesv [options]

Options:
-L ADDR    Listen on ADDR. (default `, defaultListenAddr, `)
-t DUR     Shutdown grace period. (default `, defaultGracePeriod, `)
`)
}

func bindConfigFlags(f *flag.FlagSet, c *Config) {
	f.Var(NewTextFlag(c.Listen), "L", "Listen `address`")
	f.DurationVar(&c.GracePeriod, "t", c.GracePeriod, "Shutdown grace period")
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
