package sharaq

import (
	"encoding/json"
	"fmt"
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
