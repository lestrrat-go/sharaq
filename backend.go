package sharaq

import (
	"bytes"
	"encoding/json"
	"fmt"
	"hash/crc64"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/goamz/goamz/aws"
	"github.com/goamz/goamz/s3"
)

type BackendType int

const (
	FSBackendType BackendType = iota
	S3BackendType
)

func (b *BackendType) UnmarshalJSON(data []byte) error {
	var name string
	if err := json.Unmarshal(data, &name); err != nil {
		return err
	}

	switch name {
	case "s3":
		*b = S3BackendType
		return nil
	case "fs":
		*b = FSBackendType
		return nil
	default:
		return fmt.Errorf("expected 's3' or 'fs'")
	}
}

func (b BackendType) String() string {
	switch b {
	case S3BackendType:
		return "s3"
	case FSBackendType:
		return "fs"
	default:
		return "UnknownBackend"
	}
}

func getTargetURL(r *http.Request) (*url.URL, error) {
	rawValue := r.FormValue("url")
	u, err := url.Parse(rawValue)
	if err != nil {
		return nil, err
	}

	if u.Scheme != "http" && u.Scheme != "https" {
		return nil, fmt.Errorf("scheme '%s' not supported", u.Scheme)
	}

	if u.Host == "" {
		return nil, fmt.Errorf("empty host")
	}

	return u, nil
}

type Backend interface {
	Serve(http.ResponseWriter, *http.Request)
	StoreTransformedContent(*url.URL) error
	Delete(*url.URL) error
}

func NewBackend(s *Server) (Backend, error) {
	log.Printf("Using backend type %s", s.config.BackendType())

	switch s.config.BackendType() {
	case S3BackendType:
		return NewS3Backend(s)
	case FSBackendType:
		return NewFSBackend(s)
	default:
		return nil, fmt.Errorf("Unknown backend type: %s", s.config.BackendType())
	}
}

type S3Backend struct {
	bucketName  string
	bucket      *s3.Bucket
	cache       *URLCache
	presets     map[string]string
	transformer *Transformer
}

func NewS3Backend(s *Server) (Backend, error) {
	c := s.config
	auth := aws.Auth{
		AccessKey: c.AccessKey(),
		SecretKey: c.SecretKey(),
	}

	s3o := s3.New(auth, aws.APNortheast)
	return &S3Backend{
		bucket:      s3o.Bucket(c.BucketName()),
		bucketName:  c.BucketName(),
		cache:       s.cache,
		presets:     c.Presets(),
		transformer: s.transformer,
	}, nil
}

var ErrInvalidPreset = errors.New("invalid preset parameter")
func getPresetFromRequest(r *http.Request) (string, error) {
	if preset := r.FormValue("preset"); preset != "" {
		return preset, nil
	}

	// Work with deprecated "device" parameter
	if device := r.FormValue("device"); device != "" {
		return device, nil
	}

	return "", ErrInvalidPreset
}

func (s *S3Backend) Serve(w http.ResponseWriter, r *http.Request) {
	u, err := getTargetURL(r)
	if err != nil {
		log.Printf("Bad url: %s", err)
		http.Error(w, "Bad url", 500)
		return
	}

	preset, err := getPresetFromRequest(r)
	if err != nil {
		log.Printf("Bad preset: %s", err)
		http.Error(w, "Bad preset", 500)
		return
	}

	cacheKey := MakeCacheKey("s3", preset, u.String())
	if cachedURL := s.cache.Lookup(cacheKey); cachedURL != "" {
		log.Printf("Cached entry found for %s:%s -> %s", preset, u.String(), cachedURL)
		w.Header().Add("Location", cachedURL)
		w.WriteHeader(301)
		return
	}

	// create the proper url
	specificURL := "http://" + s.bucketName + ".s3.amazonaws.com/" + preset + u.Path

	log.Printf("Making HEAD request to %s...", specificURL)
	res, err := http.Head(specificURL)
	if err != nil {
		log.Printf("Failed to make HEAD request to %s: %s", specificURL, err)
		goto FALLBACK
	}

	log.Printf("HEAD request for %s returns %d", specificURL, res.StatusCode)
	if res.StatusCode == 200 {
		go s.cache.Set(cacheKey, specificURL)
		log.Printf("HEAD request to %s was success. Redirecting to proper location", specificURL)
		w.Header().Add("Location", specificURL)
		w.WriteHeader(301)
		return
	}

	go func() {
		if err := s.StoreTransformedContent(u); err != nil {
			log.Printf("S3Backend: transformation failed: %s", err)
		}
	}()

FALLBACK:
	w.Header().Add("Location", u.String())
	w.WriteHeader(302)
}

func (s *S3Backend) StoreTransformedContent(u *url.URL) error {
	log.Printf("S3Backend: transforming image at url %s", u)

	// Transformation is completely done by the transformer, so just
	// hand it over to it
	wg := &sync.WaitGroup{}
	errCh := make(chan error, len(s.presets))
	for preset, rule := range s.presets {
		wg.Add(1)
		go func(wg *sync.WaitGroup, t *Transformer, preset string, rule string, errCh chan error) {
			defer wg.Done()

			res, err := t.Transform(rule, u.String())
			if err != nil {
				errCh <- err
				return
			}

			// good, done. save it to S3
			path := "/" + preset + u.Path
			log.Printf("Sending PUT to S3 %s...", path)
			err = s.bucket.PutReader(path, res.content, res.size, res.contentType, s3.PublicRead, s3.Options{})
			defer res.content.Close()
			if err != nil {
				errCh <- err
				return
			}
		}(wg, s.transformer, preset, rule, errCh)
	}
	wg.Wait()
	close(errCh)

	buf := &bytes.Buffer{}
	for err := range errCh {
		fmt.Fprintf(buf, "Err: %s\n", err)
	}

	if buf.Len() > 0 {
		return fmt.Errorf("error while transforming: %s", buf.String())
	}

	return nil
}

func (s *S3Backend) Delete(u *url.URL) error {
	wg := &sync.WaitGroup{}
	errCh := make(chan error, len(s.presets))
	for preset := range s.presets {
		wg.Add(1)
		go func(wg *sync.WaitGroup, preset string, errCh chan error) {
			defer wg.Done()
			path := "/" + preset + u.Path
			log.Printf(" + DELETE S3 entry %s\n", path)
			err := s.bucket.Del(path)
			if err != nil {
				errCh <- err
			}

			// fallthrough here regardless, because it's better to lose the
			// cache than to accidentally have one linger
			s.cache.Delete(MakeCacheKey(preset, u.String()))
		}(wg, preset, errCh)
	}

	wg.Wait()
	close(errCh)

	buf := &bytes.Buffer{}
	for err := range errCh {
		fmt.Fprintf(buf, "Err: %s\n", err)
	}

	if buf.Len() > 0 {
		return fmt.Errorf("error while deleting: %s", buf.String())
	}

	return nil
}

type FSBackend struct {
	root        string
	cache       *URLCache
	imageTTL    time.Duration
	presets     map[string]string
	transformer *Transformer
}

func NewFSBackend(s *Server) (Backend, error) {
	c := s.config

	root := c.StorageRoot()
	if root == "" {
		return nil, fmt.Errorf("FSBackend: 'StorageRoot' is required")
	}
	log.Printf("FSBackend: storing files under %s", root)
	return &FSBackend{
		root:        root,
		cache:       s.cache,
		imageTTL:    c.ImageTTL(),
		presets:     c.Presets(),
		transformer: s.transformer,
	}, nil
}

func (f *FSBackend) EncodeFilename(preset string, urlstr string) string {
	// we are not going to be storing the requested path directly...
	// need to encode it
	h := crc64.New(crc64Table)
	io.WriteString(h, preset)
	io.WriteString(h, urlstr)
	encodedPath := fmt.Sprintf("%x", h.Sum64())
	return filepath.Join(f.root, encodedPath[0:1], encodedPath[0:2], encodedPath[0:3], encodedPath[0:4], encodedPath)
}

func (f *FSBackend) Serve(w http.ResponseWriter, r *http.Request) {
	u, err := getTargetURL(r)
	if err != nil {
		log.Printf("Bad url: %s", err)
		http.Error(w, "Bad url", 500)
		return
	}

	preset, err := getPresetFromRequest(r)
	if err != nil {
		log.Printf("Bad preset: %s", err)
		http.Error(w, "Bad preset", 500)
		return
	}

	cacheKey := MakeCacheKey("fs", preset, u.String())
	if cachedFile := f.cache.Lookup(cacheKey); cachedFile != "" {
		log.Printf("Cached entry found for %s:%s -> %s", preset, u.String(), cachedFile)
		http.ServeFile(w, r, cachedFile)
		return
	}

	path := f.EncodeFilename(preset, u.String())
	if _, err := os.Stat(path); err == nil {
		// HIT. Serve this guy after filling the cache
		f.cache.Set(cacheKey, path)
		http.ServeFile(w, r, path)
	}

	// transformed files are not available. Let the client received the original one
	go func() {
		if err := f.StoreTransformedContent(u); err != nil {
			log.Printf("FSBackend: transformation failed: %s", err)
		}
	}()

	w.Header().Add("Location", u.String())
	w.WriteHeader(302)
}

func (f *FSBackend) StoreTransformedContent(u *url.URL) error {
	log.Printf("FSBackend: transforming image at url %s", u)

	wg := &sync.WaitGroup{}
	errCh := make(chan error, len(f.presets))
	for preset, rule := range f.presets {
		wg.Add(1)
		go func(wg *sync.WaitGroup, t *Transformer, preset string, rule string, errCh chan error) {
			defer wg.Done()

			log.Printf("FSBackend: applying transformation %s (%s)...", preset, rule)
			res, err := t.Transform(rule, u.String())
			if err != nil {
				errCh <- err
				return
			}

			path := f.EncodeFilename(preset, u.String())
			log.Printf("Saving to %s...", path)

			dir := filepath.Dir(path)
			if _, err := os.Stat(dir); err != nil {
				if err := os.MkdirAll(filepath.Dir(path), 0744); err != nil {
					errCh <- err
					return
				}
			}

			fh, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY, 0644)
			if err != nil {
				errCh <- err
				return
			}

			defer fh.Close()
			defer res.content.Close()
			if _, err := io.Copy(fh, res.content); err != nil {
				errCh <- err
				return
			}
		}(wg, f.transformer, preset, rule, errCh)
	}
	wg.Wait()
	close(errCh)

	buf := &bytes.Buffer{}
	for err := range errCh {
		fmt.Fprintf(buf, "Err: %s\n", err)
	}

	// Cleanup disk
	go f.CleanStorageRoot()

	if buf.Len() > 0 {
		return fmt.Errorf("error while transforming: %s", buf.String())
	}

	return nil
}

func (f *FSBackend) Delete(u *url.URL) error {
	wg := &sync.WaitGroup{}
	errCh := make(chan error, len(f.presets))
	for preset := range f.presets {
		wg.Add(1)
		go func(wg *sync.WaitGroup, preset string, errCh chan error) {
			defer wg.Done()
			path := f.EncodeFilename(preset, u.String())
			log.Printf(" + DELETE filesystem entry %s\n", path)
			if err := os.Remove(path); err != nil {
				errCh <- err
			}

			// fallthrough here regardless, because it's better to lose the
			// cache than to accidentally have one linger
			f.cache.Delete(MakeCacheKey("fs", preset, u.String()))
		}(wg, preset, errCh)
	}

	wg.Wait()
	close(errCh)

	buf := &bytes.Buffer{}
	for err := range errCh {
		fmt.Fprintf(buf, "Err: %s\n", err)
	}

	if buf.Len() > 0 {
		return fmt.Errorf("error while deleting: %s", buf.String())
	}

	return nil
}

func (f *FSBackend) CleanStorageRoot() error {
	if f.imageTTL <= 0 {
		return nil
	}

	filepath.Walk(f.root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}

		if time.Since(info.ModTime()) > f.imageTTL {
			os.Remove(path)
		}
		return nil
	})

	return nil
}
