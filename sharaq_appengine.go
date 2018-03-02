// +build appengine

package sharaq

import (
	"net/url"

	"github.com/pkg/errors"
	"golang.org/x/net/context"
	"google.golang.org/appengine/taskqueue"
)

var queueName = os.Getenv("SHARAQ_QUEUE_NAME")

// Under appengine, we MUST use a task queue to offload this
func (s *Server) deferedTransformAndStore(ctx context.Context, u *url.URL) error {
	task := taskqueue.NewPOSTTask("/", url.Values{
		"url": []string{u.String()},
	})
	if _, err := taskqueue.Add(ctx, task, queueName); err != nil {
		return errors.Wrap(err, `failed to add task to queue`)
	}
	return nil
}
