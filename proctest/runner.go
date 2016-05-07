package proctest

import (
	"testing"

	"golang.org/x/net/context"
	"limbo.services/proc"
)

func RunTest(t *testing.T, runner proc.Runner, f func(t *testing.T, ctx context.Context)) {
	errs := proc.Run(context.Background(),
		proc.Multi(
			runner,
			proc.Runner(func(ctx context.Context) <-chan error {
				out := make(chan error)
				go func() {
					defer close(out)
					f(t, ctx)
				}()
				return out
			})))
	for err := range errs {
		t.Error(err)
	}
}
