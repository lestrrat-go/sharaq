package sharaq

import (
	"log"
	"net"
	"net/http"
	"regexp"
	"sync"
)

type Dispatcher struct {
	backend    Backend
	listenAddr string
	bucketName string
	whitelist  []*regexp.Regexp
	cache      *URLCache
	guardian   *Guardian
}

type DispatcherConfig interface {
	DispatcherAddr() string
}

func NewDispatcher(s *Server, g *Guardian) (*Dispatcher, error) {
	c := s.config

	whitelist := make([]*regexp.Regexp, len(s.config.Whitelist()))
	for i, pat := range s.config.Whitelist() {
		re, err := regexp.Compile(pat)
		if err != nil {
			return nil, err
		}
		whitelist[i] = re
	}

	return &Dispatcher{
		backend:    s.backend,
		listenAddr: c.DispatcherAddr(),
		bucketName: c.BucketName(),
		cache:      s.cache,
		guardian:   g,
		whitelist:  whitelist,
	}, nil
}

func (d *Dispatcher) Run(doneWg *sync.WaitGroup, exitCond *sync.Cond) {
	defer doneWg.Done()

	srv := &http.Server{Addr: d.listenAddr, Handler: d}
	ln, err := makeListener(d.listenAddr)
	if err != nil {
		log.Printf("Error binding to listen address: %s", err)
		return
	}

	go func(ln net.Listener, exitCond *sync.Cond) {
		defer recover()
		exitCond.L.Lock()
		exitCond.Wait()
		exitCond.L.Unlock()
		ln.Close()
	}(ln, exitCond)

	log.Printf("Dispatcher listening on %s", d.listenAddr)
	srv.Serve(tcpKeepAliveListener{ln.(*net.TCPListener)})
}

func (d *Dispatcher) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "GET":
		d.HandleFetch(w, r)
	default:
		http.Error(w, "What, what, what?", 400)
	}
}

// HandleFetch replies with the proper URL of the image
func (d *Dispatcher) HandleFetch(w http.ResponseWriter, r *http.Request) {
	rawValue := r.FormValue("url")
	if rawValue == "" {
		http.Error(w, "Bad url", 500)
		return
	}

	allowed := false
	if len(d.whitelist) == 0 {
		allowed = true
	} else {
		for _, pat := range d.whitelist {
			if pat.MatchString(rawValue) {
				allowed = true
				break
			}
		}
	}

	if !allowed {
		http.Error(w, "Specified url not allowed", 403)
		return
	}

	d.backend.Serve(w, r)
}
