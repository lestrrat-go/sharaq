package sharaq

import (
	"net/http"
	"net/url"
	"regexp"
	"time"

	"github.com/lestrrat/sharaq/aws"
	"github.com/lestrrat/sharaq/fs"
	"github.com/lestrrat/sharaq/gcp"
	"github.com/lestrrat/sharaq/internal/context"
	"github.com/lestrrat/sharaq/internal/transformer"
	"github.com/lestrrat/sharaq/internal/urlcache"
)

type Server struct {
	backend     Backend
	config      *Config
	cache       *urlcache.URLCache
	bucketName  string
	listenAddr  string
	logConfig   *LogConfig
	transformer *transformer.Transformer
	whitelist   []*regexp.Regexp
}

type Backend interface {
	Serve(http.ResponseWriter, *http.Request)
	StoreTransformedContent(context.Context, *url.URL) error
	Delete(context.Context, *url.URL) error
}

type LogConfig struct {
	LogFile      string
	LinkName     string
	RotationTime time.Duration
	MaxAge       time.Duration
	Offset       time.Duration
}

type DispatcherConfig struct {
	Listen    string     // listen on this address. default is 0.0.0.0:9090
	AccessLog *LogConfig // dispatcher log. if nil, logs to stderr
}

type BackendConfig struct {
	Amazon     aws.Config // AWS specific config
	Type       string     // "aws" or "gcp" ("fs" for local debugging)
	FileSystem fs.Config  // File system specific config
	Google     gcp.Config // Google specific config
}

type Config struct {
	filename   string
	Backend    BackendConfig
	Debug      bool
	Dispatcher DispatcherConfig
	Presets    map[string]string
	URLCache   *urlcache.Config
	Whitelist  []string
}
