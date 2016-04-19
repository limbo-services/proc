package proc

import (
	"os"
	"os/signal"
	"reflect"

	"golang.org/x/net/context"
)

type Runner func(ctx context.Context) <-chan error

func Run(parentCtx context.Context, runners ...Runner) <-chan error {
	out := make(chan error)
	ready := make(chan struct{})

	var (
		cases       = make([]reflect.SelectCase, len(runners))
		pending     = len(runners)
		ctx, cancel = context.WithCancel(parentCtx)
	)

	go func() {
		defer close(ready)

		for i, runner := range runners {
			errChan := runner(ctx)
			cases[i] = reflect.SelectCase{
				Chan: reflect.ValueOf(errChan),
				Dir:  reflect.SelectRecv,
			}
		}
	}()

	go func() {
		defer cancel()
		defer close(out)

		var isReady bool

		for pending > 0 && isReady {
			if !isReady {
				select {
				case <-ready:
					isReady = true
				default:
				}
			}

			chosen, recv, recvOK := reflect.Select(cases)
			// log.Printf("chosen=%v, recv=%v, recvOK=%v", chosen, recv, recvOK)

			if recv.IsValid() && !recv.IsNil() {
				// error received
				err, _ := recv.Interface().(error)
				if err != nil {
					out <- err
				}
			}

			if !recvOK {
				// chanel was closed
				cancel()
				cases[chosen].Chan = reflect.Value{}
				pending--
			}
		}
	}()

	<-ready
	return out
}

func TerminateOnSignal(signals ...os.Signal) Runner {
	return func(ctx context.Context) <-chan error {
		out := make(chan error)
		go func() {
			defer close(out)

			c := make(chan os.Signal)
			defer close(c)

			go signal.Notify(c, signals...)
			defer signal.Stop(c)

			select {
			case <-c:
			case <-ctx.Done():
			}
		}()
		return out
	}
}

func Multi(runners ...Runner) Runner {
	return func(ctx context.Context) <-chan error {
		return Run(ctx, runners...)
	}
}

func Error(err error) <-chan error {
	c := make(chan error, 1)
	if err != nil {
		c <- err
	}
	close(c)
	return c
}
