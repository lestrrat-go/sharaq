package sharaq

import (
	"encoding/json"
	"hash/crc64"
	"log"
	"os"
	"sync"
)

var crc64Table *crc64.Table

func init() {
	crc64Table = crc64.MakeTable(crc64.ISO)
}

type Config struct {
	OptAccessKey      string   `json:"AccessKey"`
	OptBucketName     string   `json:"BucketName"`
	OptDispatcherAddr string   `json:"DispatcherAddr"` // listen on this address. default is 0.0.0.0:9090
	OptGuardianAddr   string   `json:"GuardianAddr"`   // listen on this address. default is 0.0.0.0:9191
	OptMemcachedAddr  []string `json:"MemcachedAddr"`
	OptSecretKey      string   `json:"SecretKey"`
	OptTransformerURL string   `json:"TransformerURL"`
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
	return nil
}

func (c Config) AccessKey() string       { return c.OptAccessKey }
func (c Config) BucketName() string      { return c.OptBucketName }
func (c Config) DispatcherAddr() string  { return c.OptDispatcherAddr }
func (c Config) GuardianAddr() string    { return c.OptGuardianAddr }
func (c Config) MemcachedAddr() []string { return c.OptMemcachedAddr }
func (c Config) SecretKey() string       { return c.OptSecretKey }
func (c Config) TransformerURL() string  { return c.OptTransformerURL }

type Server struct {
	config *Config
	cache  *URLCache
	transformer *Transformer
}

func NewServer(c *Config) *Server {
	log.Printf("Using url cache at %v", c.MemcachedAddr())
	return &Server{
		config: c,
		cache:  NewURLCache(c.MemcachedAddr()...),
	}
}

func (s *Server) Run() error {
	s.transformer = NewTransformer(s)

	g, err := NewGuardian(s)
	if err != nil {
		return err
	}

	d, err := NewDispatcher(s,g)
	if err != nil {
		return err
	}

	wg := &sync.WaitGroup{}
	wg.Add(2)

	doneCh := make(chan struct{})
	go g.Run(doneCh)
	go d.Run(doneCh)
	go func() {
		for _ = range doneCh {
			wg.Done()
		}
	}()

	wg.Wait()
	close(doneCh)

	return nil
}
