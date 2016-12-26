package fs

import (
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/lestrrat/sharaq/internal/bbpool"
	"github.com/lestrrat/sharaq/internal/context"
	"github.com/lestrrat/sharaq/internal/transformer"
	"github.com/lestrrat/sharaq/internal/urlcache"
	"github.com/lestrrat/sharaq/internal/util"
)

type Backend struct {
	root        string
	cache       *urlcache.URLCache
	imageTTL    time.Duration
	presets     map[string]string
	transformer *transformer.Transformer
}

func NewBackend(c *Config, cache *urlcache.URLCache, trans *transformer.Transformer, presets map[string]string) (*Backend, error) {
	root := c.Root
	if root == "" {
		return nil, errors.New("fs backend: 'Root' is required")
	}
	log.Printf("Backend: storing files under %s", root)
	return &Backend{
		root:        root,
		cache:       cache,
		imageTTL:    c.ImageTTL,
		presets:     presets,
		transformer: trans,
	}, nil
}

func (f *Backend) EncodeFilename(preset string, urlstr string) string {
	// we are not going to be storing the requested path directly...
	// need to encode it
	return filepath.Join(f.root, util.HashedPath(preset, urlstr))
}

func (f *Backend) Serve(w http.ResponseWriter, r *http.Request) {
	u, err := util.GetTargetURL(r)
	if err != nil {
		log.Printf("Bad url: %s", err)
		http.Error(w, "Bad url", 500)
		return
	}

	preset, err := util.GetPresetFromRequest(r)
	if err != nil {
		log.Printf("Bad preset: %s", err)
		http.Error(w, "Bad preset", 500)
		return
	}

	cacheKey := urlcache.MakeCacheKey("fs", preset, u.String())
	if cachedFile := f.cache.Lookup(util.RequestCtx(r), cacheKey); cachedFile != "" {
		log.Printf("Cached entry found for %s:%s -> %s", preset, u.String(), cachedFile)
		http.ServeFile(w, r, cachedFile)
		return
	}

	path := f.EncodeFilename(preset, u.String())
	if _, err := os.Stat(path); err == nil {
		// HIT. Serve this guy after filling the cache
		f.cache.Set(util.RequestCtx(r), cacheKey, path)
		http.ServeFile(w, r, path)
	}

	// transformed files are not available. Let the client received the original one
	go func() {
		if err := f.StoreTransformedContent(u); err != nil {
			log.Printf("Backend: transformation failed: %s", err)
		}
	}()

	w.Header().Add("Location", u.String())
	w.WriteHeader(302)
}

func (f *Backend) StoreTransformedContent(u *url.URL) error {
	log.Printf("Backend: transforming image at url %s", u)

	wg := &sync.WaitGroup{}
	errCh := make(chan error, len(f.presets))
	for preset, rule := range f.presets {
		wg.Add(1)
		go func(wg *sync.WaitGroup, t *transformer.Transformer, preset string, rule string, errCh chan error) {
			defer wg.Done()

			buf := bbpool.Get()
			defer bbpool.Release(buf)

			var res transformer.Result
			res.Content = buf

			log.Printf("Backend: applying transformation %s (%s)...", preset, rule)
			if err := t.Transform(rule, u.String(), &res); err != nil {
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
			if _, err := io.Copy(fh, buf); err != nil {
				errCh <- err
				return
			}
		}(wg, f.transformer, preset, rule, errCh)
	}
	wg.Wait()
	close(errCh)

	buf := bbpool.Get()
	defer bbpool.Release(buf)

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

func (f *Backend) Delete(u *url.URL) error {
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
			f.cache.Delete(context.Background(), urlcache.MakeCacheKey("fs", preset, u.String()))
		}(wg, preset, errCh)
	}

	wg.Wait()
	close(errCh)

	buf := bbpool.Get()
	defer bbpool.Release(buf)

	for err := range errCh {
		fmt.Fprintf(buf, "Err: %s\n", err)
	}

	if buf.Len() > 0 {
		return fmt.Errorf("error while deleting: %s", buf.String())
	}

	return nil
}

func (f *Backend) CleanStorageRoot() error {
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
