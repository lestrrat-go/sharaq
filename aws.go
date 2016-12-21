// +build aws

package sharaq

import "github.com/lestrrat/sharaq/aws"

func NewBackend(s *Server) (Backend, error) {
	return aws.NewBackend(s.config, s.cache, s.transformer)
}
