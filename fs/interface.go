package fs

import "time"

type Config struct {
	Root     string
	ImageTTL time.Duration
}
