package sharaq

import (
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"sync"

	"github.com/lestrrat/go-apache-logformat"
	"github.com/lestrrat/go-file-rotatelogs"
	"github.com/lestrrat/sharaq/internal/crc64"
)

type Guardian struct {
	backend         Backend
	listenAddr      string
	logConfig       *LogConfig
	processingMutex *sync.Mutex
	processing      map[uint64]bool
}

func NewGuardian(s *Server) (*Guardian, error) {
	c := s.config
	g := &Guardian{
		backend:         s.backend,
		listenAddr:      c.Guardian.Listen,
		logConfig:       c.Guardian.AccessLog,
		processingMutex: &sync.Mutex{},
		processing:      make(map[uint64]bool),
	}

	return g, nil
}

func (g *Guardian) Run(doneWg *sync.WaitGroup, exitCond *sync.Cond) {
	defer doneWg.Done()

	var output io.Writer = os.Stdout
	if gl := g.logConfig; gl != nil {
		glh := rotatelogs.New(
			gl.LogFile,
			rotatelogs.WithLinkName(gl.LinkName),
			rotatelogs.WithMaxAge(gl.MaxAge),
			rotatelogs.WithRotationTime(gl.RotationTime),
		)
		output = glh

		log.Printf("Guardian logging to %s", gl.LogFile)
	}
	srv := &http.Server{
		Addr:    g.listenAddr,
		Handler: apachelog.CombinedLog.Wrap(g, output),
	}

	ln, err := makeListener(g.listenAddr)
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

	log.Printf("Guardian listening on %s", g.listenAddr)
	srv.Serve(tcpKeepAliveListener{ln.(*net.TCPListener)})
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

func (g *Guardian) MarkProcessing(u *url.URL) bool {
	k := crc64.Sum(u.String())

	g.processingMutex.Lock()
	defer g.processingMutex.Unlock()
	g.processing[k] = true
	return true
}

func (g *Guardian) UnmarkProcessing(u *url.URL) {
	k := crc64.Sum(u.String())

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

	// start := time.Now()
	if err := g.backend.StoreTransformedContent(r.Context(), u); err != nil {
		log.Printf("Error detected while processing: %s", err)
		http.Error(w, err.Error(), 500)
		return
	}

	// TODO: allow configuring this later
	// w.Header().Add("X-Sharaq-Elapsed-Time", fmt.Sprintf("%0.2f", time.Since(start).Seconds()))
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

	// start := time.Now()

	if err := g.backend.Delete(r.Context(), u); err != nil {
		log.Printf("Error detected while processing: %s", err)
		http.Error(w, err.Error(), 500)
		return
	}

	// w.Header().Add("X-Sharaq-Elapsed-Time", fmt.Sprintf("%0.2f", time.Since(start).Seconds()))
}
