package sharaq

import (
	"log"
	"net/http"
	"net/url"
)

type Dispatcher struct {
	Cache *URLCache
}

func NewDispatcher() (*Dispatcher, error) {
	return &Dispatcher{
		Cache: NewURLCache("127.0.0.1:11211"),
	}, nil
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

	if cachedURL := d.Cache.Lookup(cacheKey); cachedURL != "" {
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

	if res.StatusCode == 200 {
		go d.Cache.Set(cacheKey, specificURL)
		log.Printf("HEAD request to %s was success. Redirecting to proper location", specificURL)
		w.Header().Add("Location", specificURL)
		w.WriteHeader(301)
		return
	}

	log.Printf("Requesting Guardian to store resized images...")

	// Grr, the resized image does not
	go func() {
		guardianURL := &url.URL{
			Scheme: "http",
			Host:   "127.0.0.1:9090",
		}
		query := url.Values{}
		query.Set("url", u.String())
		guardianURL.RawQuery = query.Encode()
		req, err := http.NewRequest("PUT", guardianURL.String(), nil)
		if err != nil {
			log.Printf("Failed to create request for %s: %s", guardianURL.String(), err)
			return
		}
		res, err := http.DefaultClient.Do(req)
		if err != nil {
			log.Printf("Failed to PUT to %s: %s", guardianURL.String(), err)
			return
		}
		if res.StatusCode != 200 {
			log.Printf("Failed to PUT to %s: %d", guardianURL.String(), res.StatusCode)
			return
		}
	}()

FALLBACK:
	w.Header().Add("Location", u.String())
	w.WriteHeader(302)
}