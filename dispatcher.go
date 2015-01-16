package sharaq

import (
	"log"
	"net/http"
	"net/url"
)

type Dispatcher struct {
	listenAddr string
	cache      *URLCache
	guardian   *Guardian
}

type DispatcherConfig interface {
	DispatcherAddr() string
}

func NewDispatcher(s *Server, g *Guardian) (*Dispatcher, error) {
	c := s.config
	return &Dispatcher{
		listenAddr: c.DispatcherAddr(),
		cache: s.cache,
		guardian: g,
	}, nil
}

func (d *Dispatcher) Run(doneCh chan struct{}) {
	defer func() { doneCh <- struct{}{} }()

	log.Printf("Dispatcher listening on port %s", d.listenAddr)
	http.ListenAndServe(d.listenAddr, d)
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
	base := "http://ix.peatix.com.s3.amazonaws.com"

	specificURL := base + "/" + device + u.Path

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