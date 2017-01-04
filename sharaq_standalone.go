// +build !appengine

package sharaq

import (
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	apachelog "github.com/lestrrat/go-apache-logformat"
	rotatelogs "github.com/lestrrat/go-file-rotatelogs"
	"github.com/lestrrat/go-server-starter/listener"
	"github.com/lestrrat/sharaq/internal/transformer"
	"github.com/lestrrat/sharaq/internal/urlcache"
	"github.com/pkg/errors"
	"golang.org/x/net/context"
)

func (s *Server) Run(ctx context.Context) error {
	/*
		if el := s.config.ErrorLog(); el != nil {
			elh := rotatelogs.New(
				el.LogFile,
				rotatelogs.WithLinkName(el.LinkName),
				rotatelogs.WithMaxAge(el.MaxAge),
				rotatelogs.WithRotationTime(el.RotationTime),
			)
			log.SetOutput(elh)
		}
	*/
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGHUP, syscall.SIGTERM, syscall.SIGINT, syscall.SIGQUIT)
	defer signal.Stop(sigCh)

	termLoopCh := make(chan struct{}, 1) // we keep restarting as long as there are no messages on this channel

LOOP:
	for {
		select {
		case <-termLoopCh:
			break LOOP
		default:
			// no op, but required to not block on the above case
		}

		if err := s.loopOnce(ctx, termLoopCh, sigCh); err != nil {
			log.Printf("error during loop, exiting")
			break LOOP
		}
	}
	return nil
}

func (s *Server) loopOnce(ctx context.Context, termLoopCh chan struct{}, sigCh chan os.Signal) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	var err error
	s.cache, err = urlcache.New(s.config.URLCache)
	if err != nil {
		return errors.Wrap(err, `failed to create urlcache`)
	}
	s.transformer = transformer.New()

	if err := s.newBackend(); err != nil {
		return errors.Wrap(err, `failed to create storage backend`)
	}

	done := make(chan error)
	go s.serve(ctx, done)

	select {
	case err := <-done:
		return err
	case <-ctx.Done():
		return errors.New(`context canceled`)
	case sig := <-sigCh:
		switch sig {
		case syscall.SIGHUP:
			log.Printf("Reload request received. Shutting down for reload...")
			newConfig := &Config{}
			if err := newConfig.ParseFile(s.config.filename); err != nil {
				log.Printf("Failed to reload config file %s: %s", s.config.filename, err)
			} else {
				s.config = newConfig
				if s.config.Debug {
					s.dumpConfig()
				}
			}
			// cancel so we can bail out
			cancel()
		default:
			log.Printf("Termination request received. Shutting down...")
			close(termLoopCh)
			return errors.New(`terminate`)
		}
	}

	return nil
}

// start_server support utility
func makeListener(listenAddr string) (net.Listener, error) {
	var ln net.Listener
	if listener.GetPortsSpecification() == "" {
		l, err := net.Listen("tcp", listenAddr)
		if err != nil {
			return nil, fmt.Errorf("error listening on %s: %s", listenAddr, err)
		}
		ln = l
	} else {
		ts, err := listener.Ports()
		if err != nil {
			return nil, fmt.Errorf("error parsing start_server ports: %s", err)
		}

		for _, t := range ts {
			switch t.(type) {
			case listener.TCPListener:
				tl := t.(listener.TCPListener)
				if listenAddr == fmt.Sprintf("%s:%d", tl.Addr, tl.Port) {
					ln, err = t.Listen()
					if err != nil {
						return nil, fmt.Errorf("failed to listen to start_server port: %s", err)
					}
					break
				}
			case listener.UnixListener:
				ul := t.(listener.UnixListener)
				if listenAddr == ul.Path {
					ln, err = t.Listen()
					if err != nil {
						return nil, fmt.Errorf("failed to listen to start_server port: %s", err)
					}
					break
				}
			}
		}

		if ln == nil {
			return nil, fmt.Errorf("could not find a matching listen addr between server_starter and DispatcherAddr")
		}
	}
	return ln, nil
}

// This is used in HTTP handlers to mimic+work like http.Server
type tcpKeepAliveListener struct {
	*net.TCPListener
}

func (ln tcpKeepAliveListener) Accept() (c net.Conn, err error) {
	tc, err := ln.AcceptTCP()
	if err != nil {
		return
	}
	tc.SetKeepAlive(true)
	tc.SetKeepAlivePeriod(3 * time.Minute)
	return tc, nil
}

func (s *Server) serve(ctx context.Context, done chan error) {
	defer close(done)

	var output io.Writer = os.Stdout
	if dl := s.logConfig; dl != nil {
		var options []rotatelogs.Option
		if loc := dl.Location; loc != "" {
			// TODO: Properly report errors
			l, err := time.LoadLocation(loc)
			if err == nil {
				options = append(options, rotatelogs.WithLocation(l))
			}
		}
		if name := dl.LinkName; name != "" {
			options = append(options, rotatelogs.WithLinkName(name))
		}

		if age := dl.MaxAge; age > 0 {
			options = append(options, rotatelogs.WithMaxAge(age))
		}

		if rt := dl.RotationTime; rt > 0 {
			options = append(options, rotatelogs.WithRotationTime(rt))
		}

		var err error
		output, err = rotatelogs.New(dl.LogFile, options...)
		if err != nil {
			log.Printf("Dispatcher log setup failed: %s", err)
			done <- errors.Wrap(err, `log setup failed`)
			return
		}
		log.Printf("Dispatcher logging to %s", dl.LogFile)
	}
	srv := &http.Server{
		Addr:    s.listenAddr,
		Handler: apachelog.CombinedLog.Wrap(s, output),
	}
	ln, err := makeListener(s.listenAddr)
	if err != nil {
		log.Printf("Error binding to listen address: %s", err)
		done <- errors.Wrap(err, `binding to listen address failed`)
		return
	}

	defer ln.Close()

	log.Printf("Dispatcher listening on %s", s.listenAddr)
	srv.Serve(tcpKeepAliveListener{ln.(*net.TCPListener)})
}
