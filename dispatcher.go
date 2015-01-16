package sharaq

import (
	"log"
	"net"
	"net/http"
	"net/url"
	"regexp"
	"sync"
)

type Dispatcher struct {
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
	ln, err := net.Listen("tcp", d.listenAddr)
	if err != nil {
		log.Printf("Error listening on %s: %s", d.listenAddr, err)
		return
	}
	go func(ln net.Listener, exitCond *sync.Cond) {
		defer recover()
		exitCond.L.Lock()
		exitCond.Wait()
		exitCond.L.Unlock()
		ln.Close()
	}(ln, exitCond)

	log.Printf("Dispatcher listening on port %s", d.listenAddr)
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

	u, err := url.Parse(rawValue)
	if err != nil {
		http.Error(w, "Bad url", 500)
		return
	}

	device := r.FormValue("device")

	cacheKey := MakeCacheKey(device, rawValue)

	if cachedURL := d.cache.Lookup(cacheKey); cachedURL != "" {
		log.Printf("Cached entry found for %s:%s -> %s", device, rawValue, cachedURL)
		w.Header().Add("Location", cachedURL)
		w.WriteHeader(301)
		return
	}

	// create the proper url
	specificURL := "http://" + d.bucketName + ".s3.amazonaws.com/" + device + u.Path

	log.Printf("Making HEAD request to %s...", specificURL)
	res, err := http.Head(specificURL)
	if err != nil {
		log.Printf("Failed to make HEAD request to %s: %s", specificURL, err)
		goto FALLBACK
	}

	log.Printf("HEAD request for %s returns %d", specificURL, res.StatusCode)
	if res.StatusCode == 200 {
		go d.cache.Set(cacheKey, specificURL)
		log.Printf("HEAD request to %s was success. Redirecting to proper location", specificURL)
		w.Header().Add("Location", specificURL)
		w.WriteHeader(301)
		return
	}

	log.Printf("Requesting Guardian to store resized images...")
	go d.guardian.transformAllAndStore(u)

FALLBACK:
	w.Header().Add("Location", u.String())
	w.WriteHeader(302)
}
