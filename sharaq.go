package sharaq

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/lestrrat/go-server-starter/listener"
	"github.com/lestrrat/sharaq/aws"
	"github.com/lestrrat/sharaq/fs"
	"github.com/lestrrat/sharaq/gcp"
	"github.com/lestrrat/sharaq/internal/transformer"
	"github.com/lestrrat/sharaq/internal/urlcache"
	"github.com/pkg/errors"
)

type LogConfig struct {
	LogFile      string
	LinkName     string
	RotationTime time.Duration
	MaxAge       time.Duration
	Offset       time.Duration
}

type DispatcherConfig struct {
	Listen    string     // listen on this address. default is 0.0.0.0:9090
	AccessLog *LogConfig // dispatcher log. if nil, logs to stderr
}

type GuardianConfig struct {
	Listen    string     // listen on this address. default is 0.0.0.0:9191
	AccessLog *LogConfig // dispatcher log. if nil, logs to stderr
}

type BackendConfig struct {
	Amazon     aws.Config // AWS specific config
	Type       string     // "aws" or "gcp" ("fs" for local debugging)
	FileSystem fs.Config  // File system specific config
	Google     gcp.Config // Google specific config
}

type Config struct {
	filename   string
	Backend    BackendConfig
	Debug      bool
	Dispatcher DispatcherConfig
	Guardian   GuardianConfig
	Presets    map[string]string
	URLCache   *urlcache.Config
	Whitelist  []string
}

func NewServer(c *Config) *Server {
	s := &Server{
		config: c,
	}
	if s.config.Debug {
		s.dumpConfig()
	}
	return s
}

func (s *Server) dumpConfig() {
	j, err := json.MarshalIndent(s.config, "", "  ")
	if err != nil {
		return
	}

	scanner := bufio.NewScanner(bytes.NewBuffer(j))
	for scanner.Scan() {
		l := scanner.Text()
		log.Print(l)
	}
}

func (s *Server) newBackend() error {
	switch s.config.Backend.Type {
	case "aws":
		b, err := aws.NewBackend(
			&s.config.Backend.Amazon,
			s.cache,
			s.transformer,
			s.config.Presets,
		)
		if err != nil {
			return errors.Wrap(err, `failed to create aws backend`)
		}
		s.backend = b
	case "gcp":
		b, err := gcp.NewBackend(
			&s.config.Backend.Google,
			s.cache,
			s.transformer,
			s.config.Presets,
		)
		if err != nil {
			return errors.Wrap(err, `failed to create gcp backend`)
		}
		s.backend = b
	default:
		return errors.Errorf(`invalid storage backend %s`, s.config.Backend.Type)
	}
	return nil
}

func (s *Server) Run() error {
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

	var err error
LOOP:
	for {
		select {
		case <-termLoopCh:
			break LOOP
		default:
			// no op, but required to not block on the above case
		}

		s.cache, err = urlcache.New(s.config.URLCache)
		if err != nil {
			return errors.Wrap(err, `failed to create urlcache`)
		}
		s.transformer = transformer.New()

		if err := s.newBackend(); err != nil {
			return errors.Wrap(err, `failed to create storage backend`)
		}

		g, err := NewGuardian(s)
		if err != nil {
			return err
		}

		d, err := NewDispatcher(s, g)
		if err != nil {
			return err
		}

		exitCond := sync.NewCond(&sync.RWMutex{})
		go func(c *sync.Cond) {
			sig := <-sigCh
			c.Broadcast()

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
			default:
				log.Printf("Termination request received. Shutting down...")
				close(termLoopCh)
			}
		}(exitCond)

		wg := &sync.WaitGroup{}
		wg.Add(2)

		go g.Run(wg, exitCond)
		go d.Run(wg, exitCond)

		wg.Wait()
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
