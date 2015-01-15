package sharaq

import (
	"bytes"
	"fmt"
	"hash/crc64"
	"io"
	"log"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/goamz/goamz/aws"
	"github.com/goamz/goamz/s3"
)

type Guardian struct {
	TransformerURL  string
	Bucket          *s3.Bucket
	processingMutex *sync.Mutex
	processing      map[uint64]bool
}

type GuardianConfig interface {
	TransformerURL() string
	BucketName() string
	AccessKey() string
	SecretKey() string
}

var presets = map[string]string{
	"smartphone": "600x1000,fit",
	"tablet":     "1000x2000,fit",
}

func NewGuardian(c GuardianConfig) (*Guardian, error) {
	auth := aws.Auth{
		AccessKey: c.AccessKey(),
		SecretKey: c.SecretKey(),
	}

	s := s3.New(auth, aws.APNortheast)
	g := &Guardian{
		TransformerURL:  c.TransformerURL(),
		Bucket:          s.Bucket(c.BucketName()),
		processingMutex: &sync.Mutex{},
		processing:      make(map[uint64]bool),
	}

	return g, nil
}

func (g *Guardian) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "PUT":
		g.HandleStore(w, r)
	case "DELETE":
		g.HandleDelete(w, r)
	default:
		http.Error(w, "What, what, what?", 400)
	}
}

// Very silly locking, and storage. fix me later
var crc64Table = crc64.MakeTable(crc64.ISO)

func (g *Guardian) MarkProcessing(u *url.URL) bool {
	h := crc64.New(crc64Table)
	io.WriteString(h, u.String())
	k := h.Sum64()

	g.processingMutex.Lock()
	defer g.processingMutex.Unlock()
	g.processing[k] = true
	return true
}

func (g *Guardian) UnmarkProcessing(u *url.URL) {
	h := crc64.New(crc64Table)
	io.WriteString(h, u.String())
	k := h.Sum64()

	g.processingMutex.Lock()
	defer g.processingMutex.Unlock()
	delete(g.processing, k)
}

// HandleStore accepts PUT requests to create resized images and
// store them on S3
func (g *Guardian) HandleStore(w http.ResponseWriter, r *http.Request) {
	rawValue := r.FormValue("url")
	if rawValue == "" {
		log.Printf("URL was empty")
		http.Error(w, "Bad url", 500)
		return
	}

	u, err := url.Parse(rawValue)
	if err != nil {
		log.Printf("Parsing URL '%s' failed: %s", rawValue, err)
		http.Error(w, "Bad url", 500)
		return
	}

	// Don't process the same url while somebody else is processing it
	if !g.MarkProcessing(u) {
		log.Printf("URL '%s' is being processed", rawValue)
		http.Error(w, "url is being processed", 500)
		return
	}
	defer g.UnmarkProcessing(u)

	start := time.Now()
	// Transformation is completely done by the transformer, so just
	// hand it over to it
	wg := &sync.WaitGroup{}
	errCh := make(chan error, len(presets))
	for preset, rule := range presets {
		xformURL := g.TransformerURL + "/" + rule + "/" + u.String()
		wg.Add(1)
		go func(wg *sync.WaitGroup, targetURL string, preset string, errCh chan error) {
			defer wg.Done()

			log.Printf("Resizing image via %s...", targetURL)
			res, err := http.Get(targetURL)
			if err != nil {
				errCh <- err
				return
			}

			if res.StatusCode != 200 {
				errCh <- fmt.Errorf("url: %s, got %d", targetURL, res.StatusCode)
				return
			}

			// good, done. save it to S3
			path := "/"+preset+u.Path
			log.Printf("Sending PUT to S3 %s...", path)
			err = g.Bucket.PutReader(path, res.Body, res.ContentLength, res.Header.Get("Content-Type"), s3.PublicRead, s3.Options{})
			defer res.Body.Close()
			if err != nil {
				errCh <- err
			}
		}(wg, xformURL, preset, errCh)
	}

	wg.Wait()
	close(errCh)

	buf := &bytes.Buffer{}
	for err := range errCh {
		fmt.Fprintf(buf, "Err: %s\n", err)
	}

	if buf.Len() > 0 {
		log.Printf("Error detected while processing: %s", buf.String())
		http.Error(w, buf.String(), 500)
		return
	}

	w.Header().Add("X-Peatix-Elapsed-Time", fmt.Sprintf("%0.2f", time.Since(start).Seconds()))
}

// HandleDelete accepts DELETE requests to delete all known resized images from S3
func (g *Guardian) HandleDelete(w http.ResponseWriter, r *http.Request) {
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

	// Don't process the same url while somebody else is processing it
	if !g.MarkProcessing(u) {
		http.Error(w, "url is being processed", 500)
		return
	}
	defer g.UnmarkProcessing(u)

	log.Printf("DELETE for source image: %s\n", u.String())

	start := time.Now()
	// Transformation is completely done by the transformer, so just
	// hand it over to it
	wg := &sync.WaitGroup{}
	errCh := make(chan error, len(presets))
	for preset := range presets {
		wg.Add(1)
		go func(wg *sync.WaitGroup, preset string, errCh chan error) {
			defer wg.Done()
			path := "/" + preset + u.Path
			log.Printf(" + DELETE S3 entry %s\n", path)
			err = g.Bucket.Del(path)
			if err != nil {
				errCh <- err
			}
		}(wg, preset, errCh)
	}

	wg.Wait()
	close(errCh)

	buf := &bytes.Buffer{}
	for err := range errCh {
		fmt.Fprintf(buf, "Err: %s\n", err)
	}

	if buf.Len() > 0 {
		http.Error(w, buf.String(), 500)
		return
	}

	w.Header().Add("X-Peatix-Elapsed-Time", fmt.Sprintf("%0.2f", time.Since(start).Seconds()))
}