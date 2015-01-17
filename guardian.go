package sharaq

import (
	"fmt"
	"hash/crc64"
	"html/template"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"sync"
	"time"
)

type Guardian struct {
	backend         Backend
	listenAddr      string
	processingMutex *sync.Mutex
	processing      map[uint64]bool
}

var presets = map[string]string{
	"pc-thumb":     "360x216",
	"ticket-thumb": "170x230",
	"wando-thumb":  "596x450",
	"email-thumb":  "596x450",
}

func NewGuardian(s *Server) (*Guardian, error) {
	c := s.config
	g := &Guardian{
		listenAddr:      c.GuardianAddr(),
		processingMutex: &sync.Mutex{},
		processing:      make(map[uint64]bool),
	}

	return g, nil
}

func (g *Guardian) Run(doneWg *sync.WaitGroup, exitCond *sync.Cond) {
	defer doneWg.Done()

	srv := &http.Server{Addr: g.listenAddr, Handler: g}
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
	case "GET":
		g.HandleView(w, r)
	case "PUT":
		g.HandleStore(w, r)
	case "DELETE":
		g.HandleDelete(w, r)
	default:
		http.Error(w, "What, what, what?", 400)
	}
}

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

func (g *Guardian) HandleView(w http.ResponseWriter, r *http.Request) {
	rawValue := r.FormValue("url")
	if rawValue == "" {
		log.Printf("URL was empty")
		http.Error(w, "Bad url", 500)
		return
	}

	vars := struct {
		Images map[string]string
	}{
		Images: make(map[string]string),
	}
	for name := range presets {
		vars.Images[name] = "" // "http://" + g.bucketName + ".s3.amazonaws.com/" + name + u.Path
	}

	t, err := template.New("sharaq-view").Parse(`
<html>
<body>
<table>
{{range $name, $url := .Images}}
<tr>
    <td>{{ $name }}</td>
    <td><img src="{{ $url }}"></td>
</tr>
{{end}}
</table>
</body>
</html>`)
	if err != nil {
		log.Printf("Error parsing template: %s", err)
		http.Error(w, "Template error", 500)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf8")
	t.Execute(w, vars)
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
	if err := g.backend.StoreTransformedContent(u); err != nil {
		log.Printf("Error detected while processing: %s", err)
		http.Error(w, err.Error(), 500)
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

	if err := g.backend.Delete(u); err != nil {
		log.Printf("Error detected while processing: %s", err)
		http.Error(w, err.Error(), 500)
		return
	}

	w.Header().Add("X-Peatix-Elapsed-Time", fmt.Sprintf("%0.2f", time.Since(start).Seconds()))
}
