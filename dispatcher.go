package sharaq

import (
	"context"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"sync"
	"time"

	"github.com/lestrrat/go-apache-logformat"
	"github.com/lestrrat/go-file-rotatelogs"
	"github.com/lestrrat/sharaq/internal/urlcache"
	"github.com/lestrrat/sharaq/internal/util"
	"github.com/pkg/errors"
)

func NewDispatcher(s *Server) (*Dispatcher, error) {
	c := s.config

	whitelist := make([]*regexp.Regexp, len(s.config.Whitelist))
	for i, pat := range s.config.Whitelist {
		re, err := regexp.Compile(pat)
		if err != nil {
			return nil, err
		}
		whitelist[i] = re
	}

	return &Dispatcher{
		backend:    s.backend,
		listenAddr: c.Dispatcher.Listen,
		cache:      s.cache,
		logConfig:  c.Dispatcher.AccessLog,
		whitelist:  whitelist,
	}, nil
}

func (d *Dispatcher) Run(doneWg *sync.WaitGroup, exitCond *sync.Cond) {
	defer doneWg.Done()

	var output io.Writer = os.Stdout
	if dl := d.logConfig; dl != nil {
		dlh := rotatelogs.New(
			dl.LogFile,
			rotatelogs.WithLinkName(dl.LinkName),
			rotatelogs.WithMaxAge(dl.MaxAge),
			rotatelogs.WithRotationTime(dl.RotationTime),
		)
		output = dlh

		log.Printf("Dispatcher logging to %s", dl.LogFile)
	}
	srv := &http.Server{
		Addr:    d.listenAddr,
		Handler: apachelog.CombinedLog.Wrap(d, output),
	}
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
		d.handleFetch(w, r)
	case "PUT":
		d.handleStore(w, r)
	case "DELETE":
		d.handleDelete(w, r)
	default:
		http.Error(w, "What, what, what?", 400)
	}
}

// handleFetch replies with the proper URL of the image
func (d *Dispatcher) handleFetch(w http.ResponseWriter, r *http.Request) {
	u, err := util.GetTargetURL(r)
	if err != nil {
		http.Error(w, `url parameter missing`, http.StatusBadRequest)
		return
	}

	allowed := false
	if len(d.whitelist) == 0 {
		allowed = true
	} else {
		for _, pat := range d.whitelist {
			if pat.MatchString(u.String()) {
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

func (d *Dispatcher) markProcessing(ctx context.Context, u *url.URL) error {
	cacheKey := urlcache.MakeCacheKey("processing", u.String())
	return errors.Wrap(
		d.cache.SetNX(ctx, cacheKey, "XXX", urlcache.WithExpires(5*time.Second)),
		`failed to set cache`,
	)
}

func (d *Dispatcher) unmarkProcessing(ctx context.Context, u *url.URL) error {
	cacheKey := urlcache.MakeCacheKey("processing", u.String())
	return errors.Wrap(
		d.cache.Delete(ctx, cacheKey),
		`failed to delete cache`,
	)
}

// handleStore accepts PUT requests to create resized images and
// store them on S3
func (d *Dispatcher) handleStore(w http.ResponseWriter, r *http.Request) {
	u, err := util.GetTargetURL(r)
	if err != nil {
		http.Error(w, `url parameter missing`, http.StatusBadRequest)
		return
	}

	ctx := r.Context()
	// Don't process the same url while somebody else is processing it
	if err := d.markProcessing(ctx, u); err != nil {
		log.Printf("URL '%s' is being processed", u.String())
		http.Error(w, "url is being processed", 500)
		return
	}
	defer d.unmarkProcessing(ctx, u)

	// start := time.Now()
	if err := d.backend.StoreTransformedContent(ctx, u); err != nil {
		log.Printf("Error detected while processing: %s", err)
		http.Error(w, err.Error(), 500)
		return
	}

	// TODO: allow configuring this later
	// w.Header().Add("X-Sharaq-Elapsed-Time", fmt.Sprintf("%0.2f", time.Since(start).Seconds()))
}

// handleDelete accepts DELETE requests to delete all known resized images
func (d *Dispatcher) handleDelete(w http.ResponseWriter, r *http.Request) {
	u, err := util.GetTargetURL(r)
	if err != nil {
		http.Error(w, `url parameter missing`, http.StatusBadRequest)
		return
	}

	ctx := r.Context()

	// Don't process the same url while somebody else is processing it
	if err := d.markProcessing(ctx, u); err != nil {
		http.Error(w, "url is being processed", 500)
		return
	}
	defer d.unmarkProcessing(ctx, u)

	if err := d.backend.Delete(ctx, u); err != nil {
		log.Printf("Error detected while processing: %s", err)
		http.Error(w, err.Error(), 500)
		return
	}

	// w.Header().Add("X-Sharaq-Elapsed-Time", fmt.Sprintf("%0.2f", time.Since(start).Seconds()))
}
