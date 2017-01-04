package gcp

import (
	"io"
	"net/http"
	"net/url"
	"path"

	"cloud.google.com/go/storage"
	"golang.org/x/net/context"
	"golang.org/x/oauth2/google"
	"golang.org/x/sync/errgroup"
	"google.golang.org/api/option"

	"github.com/lestrrat/sharaq/internal/bbpool"
	"github.com/lestrrat/sharaq/internal/errors"
	"github.com/lestrrat/sharaq/internal/log"
	"github.com/lestrrat/sharaq/internal/transformer"
	"github.com/lestrrat/sharaq/internal/urlcache"
	"github.com/lestrrat/sharaq/internal/util"
)

type StorageBackend struct {
	bucketName  string
	cache       *urlcache.URLCache
	prefix      string
	presets     map[string]string
	transformer *transformer.Transformer
}

type redirectContent string

func (s redirectContent) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	log.Debugf(util.RequestCtx(r), "Object %s exists. Redirecting to proper location", string(s))
	w.Header().Add("Location", string(s))
	w.WriteHeader(http.StatusMovedPermanently)
}

func NewBackend(c *Config, cache *urlcache.URLCache, trans *transformer.Transformer, presets map[string]string) (*StorageBackend, error) {
	return &StorageBackend{
		bucketName:  c.BucketName,
		cache:       cache,
		prefix:      c.Prefix,
		presets:     presets,
		transformer: trans,
	}, nil
}

func (s *StorageBackend) getClient(ctx context.Context) (*storage.Client, error) {
	tokesrc, err := google.DefaultTokenSource(ctx, storage.ScopeFullControl)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get default token source for storage client")
	}

	client, err := storage.NewClient(ctx, option.WithTokenSource(tokesrc))
	if err != nil {
		return nil, errors.Wrap(err, "failed to create storage client")
	}
	return client, nil
}

func (s *StorageBackend) Get(ctx context.Context, u *url.URL, preset string) (http.Handler, error) {
	cacheKey := urlcache.MakeCacheKey("gcp", preset, u.String())
	if cachedURL := s.cache.Lookup(ctx, cacheKey); cachedURL != "" {
		log.Debugf(ctx, "Cached entry found for %s:%s -> %s", preset, u.String(), cachedURL)
		return redirectContent(cachedURL), nil
	}

	cl, err := s.getClient(ctx)
	if err != nil {
		return nil, errors.Wrap(err, `failed to create client`)
	}

	if _, err := cl.Bucket(s.bucketName).Object(s.makeStoragePath(preset, u)).Attrs(ctx); err != nil {
		return nil, errors.TransformationRequiredError{}
	}

	specificURL := u.Scheme + "://storage.googleapis.com/" + s.makeStoragePath(preset, u)
	return redirectContent(specificURL), nil
}

func (s *StorageBackend) makeStoragePath(preset string, u *url.URL) string {
	list := make([]string, 0, 4)
	if s.prefix != "" {
		list = append(list, s.prefix)
	}
	list = append(list, preset, u.Host, u.Path)
	return path.Join(list...)
}

func (s *StorageBackend) StoreTransformedContent(ctx context.Context, u *url.URL) error {
	log.Debugf(ctx, "StorageBackend: transforming image at url %s", u)

	cl, err := s.getClient(ctx)
	if err != nil {
		return errors.Wrap(err, `failed to get client for Store`)
	}

	bkt := cl.Bucket(s.bucketName)

	var grp *errgroup.Group
	grp, ctx = errgroup.WithContext(ctx)

	// Transformation is completely done by the transformer, so just
	// hand it over to it
	for preset, rule := range s.presets {
		t := s.transformer
		preset := preset
		rule := rule
		grp.Go(func() error {
			buf := bbpool.Get()
			defer bbpool.Release(buf)

			var res transformer.Result
			res.Content = buf

			err := t.Transform(rule, u.String(), &res)
			if err != nil {
				return errors.Wrap(err, `failed to transform image`)
			}

			// good, done. save it to Google Storage
			p := s.makeStoragePath(preset, u)
			log.Debugf(ctx, "Writing to Google Storage %s...", p)

			wc := bkt.Object(p).NewWriter(ctx)

			wc.ContentType = res.ContentType
			wc.ACL = []storage.ACLRule{
				{storage.AllUsers, storage.RoleReader},
			}

			if _, err := io.Copy(wc, buf); err != nil {
				return errors.Wrapf(err, `failed to write data to %s`, p)
			}

			if err := wc.Close(); err != nil {
				return errors.Wrap(err, `failed to properly close writer for google storage`)
			}
			cacheKey := urlcache.MakeCacheKey("gcY", preset, u.String())
			specificURL := u.Scheme + "://storage.googleapis.com/" + s.makeStoragePath(preset, u)
			s.cache.Set(ctx, cacheKey, specificURL)
			return nil
		})
	}
	return grp.Wait()
}

func (s *StorageBackend) Delete(ctx context.Context, u *url.URL) error {
	cl, err := s.getClient(ctx)
	if err != nil {
		return errors.Wrap(err, `failed to get client for Delete`)
	}

	bkt := cl.Bucket(s.bucketName)

	var grp *errgroup.Group
	grp, ctx = errgroup.WithContext(ctx)

	for preset := range s.presets {
		preset := preset
		grp.Go(func() error {
			// delete the cache regardless, because it's better to lose the
			// cache than to accidentally have one linger
			defer s.cache.Delete(ctx, urlcache.MakeCacheKey(preset, u.String()))

			p := s.makeStoragePath(preset, u)
			log.Debugf(ctx, " + DELETE Google Storage entry %s\n", p)
			return bkt.Object(p).Delete(ctx)
		})
	}

	return errors.Wrap(grp.Wait(), `deleting from google storage`)
}
