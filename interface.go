package sharaq

import (
	"net/http"
	"net/url"
	"regexp"
	"time"

	"github.com/lestrrat/sharaq/aws"
	"github.com/lestrrat/sharaq/fs"
	"github.com/lestrrat/sharaq/gcp"
	"github.com/lestrrat/sharaq/internal/transformer"
	"github.com/lestrrat/sharaq/internal/urlcache"
	"golang.org/x/net/context"
)

type Server struct {
	backend     Backend
	config      *Config
	cache       *urlcache.URLCache
	bucketName  string
	logConfig   *LogConfig
	tokens      map[string]struct{} // tokens required to accept administrative requests
	transformer *transformer.Transformer
	whitelist   []*regexp.Regexp
}

type Backend interface {
	Get(context.Context, *url.URL, string) (http.Handler, error)
	StoreTransformedContent(context.Context, *url.URL) error
	Delete(context.Context, *url.URL) error
}

type LogConfig struct {
	LogFile      string
	LinkName     string
	RotationTime time.Duration
	MaxAge       time.Duration
	Location     string
}

type BackendConfig struct {
	Amazon     aws.Config // AWS specific config
	Type       string     // "aws" or "gcp" ("fs" for local debugging)
	FileSystem fs.Config  // File system specific config
	Google     gcp.Config `env:"gcp"` // Google specific config
}

type Config struct {
	filename  string
	AccessLog *LogConfig // access log. if nil, logs to stderr
	Backend   BackendConfig
	Debug     bool
	Listen    string // listen on this address. default is 0.0.0.0:9090
	Presets   map[string]string
	Tokens    []string
	URLCache  *urlcache.Config
	Whitelist []string
}
