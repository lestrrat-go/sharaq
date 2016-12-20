package sharaq

import "testing"

func TestBackendSerialization(t *testing.T) {
	c := &Config{}
	b := bbpool.Get()
	defer bbpool.Release(b)
	b.WriteString(`{"BackendType":"s3"}`)
	c.Parse(b)

	if c.BackendType() != S3BackendType {
		t.Errorf("Expected S3BackendType, got %s", c.BackendType())
	}
}
