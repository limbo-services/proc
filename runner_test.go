package proc

import (
	"testing"
	"time"

	"golang.org/x/net/context"
)

func TestRun(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	t.Log("starting Root")
	errs := Run(ctx,
		func(ctx context.Context) <-chan error {
			out := make(chan error)

			t.Log("starting A")
			time.Sleep(5 * time.Second)
			t.Log("started A")

			go func() {
				defer close(out)

				t.Log("running A")
				time.Sleep(5 * time.Second)
				t.Log("ran A")
			}()
			return out
		},
		func(ctx context.Context) <-chan error {
			out := make(chan error)

			t.Log("starting B")
			time.Sleep(2 * time.Second)
			t.Log("started B")

			go func() {
				defer close(out)

				t.Log("running B")
				time.Sleep(5 * time.Second)
				t.Log("ran B")
			}()
			return out
		})
	t.Log("started Root")

	t.Log("running Root")
	for err := range errs {
		t.Logf("errors: %s", err)
	}
	t.Log("ran Root")
}
