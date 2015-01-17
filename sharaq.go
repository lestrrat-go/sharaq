package sharaq

import (
	"encoding/json"
	"fmt"
	"hash/crc64"
	"log"
	"net"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/lestrrat/go-server-starter/listener"
)

var crc64Table *crc64.Table

func init() {
	crc64Table = crc64.MakeTable(crc64.ISO)
}

type Config struct {
	filename          string
	OptAccessKey      string      `json:"AccessKey"`
	OptBackendType    BackendType `json:"Backend"`
	OptBucketName     string      `json:"BucketName"`
	OptDispatcherAddr string      `json:"DispatcherAddr"` // listen on this address. default is 0.0.0.0:9090
	OptGuardianAddr   string      `json:"GuardianAddr"`   // listen on this address. default is 0.0.0.0:9191
	OptMemcachedAddr  []string    `json:"MemcachedAddr"`
	OptSecretKey      string      `json:"SecretKey"`
	OptStorageRoot    string      `json:"StorageRoot"`
	OptWhitelist      []string    `json:"Whitelist"`
}

func (c *Config) ParseFile(f string) error {
	fh, err := os.Open(f)
	if err != nil {
		return err
	}
	defer fh.Close()

	dec := json.NewDecoder(fh)
	if err = dec.Decode(c); err != nil {
		return err
	}

	if c.OptDispatcherAddr == "" {
		c.OptDispatcherAddr = "0.0.0.0:9090"
	}
	if c.OptGuardianAddr == "" {
		c.OptGuardianAddr = "0.0.0.0:9191"
	}
	if len(c.OptMemcachedAddr) < 1 {
		c.OptMemcachedAddr = []string{"127.0.0.1:11211"}
	}
	c.filename = f

	// Normalize shorthand form to full form
	if c.OptDispatcherAddr[0] == ':' {
		c.OptDispatcherAddr = "0.0.0.0" + c.OptDispatcherAddr
	}

	if c.OptGuardianAddr[0] == ':' {
		c.OptGuardianAddr = "0.0.0.0" + c.OptGuardianAddr
	}

	return nil
}

func (c Config) AccessKey() string        { return c.OptAccessKey }
func (c Config) BackendType() BackendType { return c.OptBackendType }
func (c Config) BucketName() string       { return c.OptBucketName }
func (c Config) DispatcherAddr() string   { return c.OptDispatcherAddr }
func (c Config) GuardianAddr() string     { return c.OptGuardianAddr }
func (c Config) MemcachedAddr() []string  { return c.OptMemcachedAddr }
func (c Config) SecretKey() string        { return c.OptSecretKey }
func (c Config) StorageRoot() string      { return c.OptStorageRoot }
func (c Config) Whitelist() []string      { return c.OptWhitelist }

type Server struct {
	backend     Backend
	config      *Config
	cache       *URLCache
	transformer *Transformer
}

func NewServer(c *Config) *Server {
	log.Printf("Using url cache at %v", c.MemcachedAddr())
	return &Server{
		config: c,
	}
}

func (s *Server) Run() error {
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

		s.cache = NewURLCache(s.config.MemcachedAddr()...)
		s.transformer = NewTransformer(s)
		s.backend = NewBackend(s)

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
