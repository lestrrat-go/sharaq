package sharaq

import (
	"bufio"
	"bytes"
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

	"github.com/lestrrat/go-file-rotatelogs"
	"github.com/lestrrat/go-server-starter/listener"
)

var crc64Table *crc64.Table

func init() {
	crc64Table = crc64.MakeTable(crc64.ISO)
}

type LogConfig struct {
	LogFile      string
	LinkName     string
	RotationTime time.Duration
	MaxAge       time.Duration
	Offset       time.Duration
}

type Config struct {
	filename          string
	OptAccessKey      string            `json:"AccessKey"`
	OptBackendType    BackendType       `json:"Backend"`
	OptBucketName     string            `json:"BucketName"`
	OptDebug          bool              `json:"Debug"`
	OptDispatcherAddr string            `json:"DispatcherAddr"` // listen on this address. default is 0.0.0.0:9090
	OptDispatcherLog  *LogConfig        `json:"DispatcherLog"`  // dispatcher log. if nil, logs to stderr
	OptErrorLog       *LogConfig        `json:"ErrorLog"`       // error log location. if nil, logs to stderr
	OptGuardianAddr   string            `json:"GuardianAddr"`   // listen on this address. default is 0.0.0.0:9191
	OptGuardianLog    *LogConfig        `json:"GuardianLog"`
	OptMemcachedAddr  []string          `json:"MemcachedAddr"`
	OptPresets        map[string]string `json:"Presets"`
	OptSecretKey      string            `json:"SecretKey"`
	OptStorageRoot    string            `json:"StorageRoot"`
	OptWhitelist      []string          `json:"Whitelist"`
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

	if len(c.OptPresets) == 0 {
		return fmt.Errorf("error: Presets is empty")
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

	applyLogDefaults := func(c *LogConfig) {
		if c.RotationTime <= 0 {
			// 1 day
			c.RotationTime = 24 * time.Hour
		}
		if c.MaxAge <= 0 {
			// 30 days
			c.MaxAge = 30 * 24 * time.Hour
		}
	}
	if c.OptErrorLog != nil {
		applyLogDefaults(c.OptErrorLog)
	}
	if c.OptDispatcherLog != nil {
		applyLogDefaults(c.OptDispatcherLog)
	}
	if c.OptGuardianLog != nil {
		applyLogDefaults(c.OptGuardianLog)
	}

	return nil
}

func (c Config) AccessKey() string          { return c.OptAccessKey }
func (c Config) BackendType() BackendType   { return c.OptBackendType }
func (c Config) BucketName() string         { return c.OptBucketName }
func (c Config) Debug() bool                { return c.OptDebug }
func (c Config) DispatcherAddr() string     { return c.OptDispatcherAddr }
func (c Config) DispatcherLog() *LogConfig  { return c.OptDispatcherLog }
func (c Config) ErrorLog() *LogConfig       { return c.OptErrorLog }
func (c Config) GuardianAddr() string       { return c.OptGuardianAddr }
func (c Config) GuardianLog() *LogConfig    { return c.OptGuardianLog }
func (c Config) MemcachedAddr() []string    { return c.OptMemcachedAddr }
func (c Config) Presets() map[string]string { return c.OptPresets }
func (c Config) SecretKey() string          { return c.OptSecretKey }
func (c Config) StorageRoot() string        { return c.OptStorageRoot }
func (c Config) Whitelist() []string        { return c.OptWhitelist }

type Server struct {
	backend     Backend
	config      *Config
	cache       *URLCache
	transformer *Transformer
}

func NewServer(c *Config) *Server {
	s := &Server{
		config: c,
	}
	if s.config.Debug() {
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

func (s *Server) Run() error {
	if el := s.config.ErrorLog(); el != nil {
		elh := rotatelogs.NewRotateLogs(el.LogFile)
		elh.LinkName = el.LinkName
		elh.MaxAge = el.MaxAge
		elh.Offset = el.Offset
		elh.RotationTime = el.RotationTime
		log.SetOutput(elh)
	}
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

		log.Printf("Using url cache at %v", s.config.MemcachedAddr())
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
					if s.config.Debug() {
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
