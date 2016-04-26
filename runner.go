package proc // import "limbo.services/proc"

import (
	"os"
	"os/signal"
	"reflect"

	"golang.org/x/net/context"
)

type Runner func(ctx context.Context) <-chan error

func Run(parentCtx context.Context, runners ...Runner) <-chan error {
	var (
		out         = make(chan error)
		ready       = make(chan struct{})
		casesC      = make(chan (<-chan error), len(runners))
		ctx, cancel = context.WithCancel(parentCtx)
	)

	go func() {
		defer close(casesC)
		for _, runner := range runners {
			casesC <- runner(ctx)
		}
	}()

	go func(out chan<- error) {
		defer cancel()
		defer close(out)

		var (
			pending int
			cases   = make([]reflect.SelectCase, 0, len(runners)+1)
		)

		cases = append(cases, reflect.SelectCase{
			Chan: reflect.ValueOf(casesC),
			Dir:  reflect.SelectRecv,
		})

		for {

			if pending <= 0 && !cases[0].Chan.IsValid() {
				return
			}

			chosen, recv, recvOK := reflect.Select(cases)

			if chosen == 0 {
				if recv.IsValid() && !recv.IsNil() {
					pending++
					cases = append(cases, reflect.SelectCase{
						Chan: recv,
						Dir:  reflect.SelectRecv,
					})
				}

				if !recvOK {
					cases[chosen].Chan = reflect.Value{}
					close(ready)
				}

				continue
			}

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
	}(out)

	// buffer boot errors
	var bootErrors []error
BOOT_LOOP:
	for {
		select {
		case <-ready:
			break BOOT_LOOP
		case err, ok := <-out:
			if !ok {
				out = nil
			}
			if err != nil {
				bootErrors = append(bootErrors, err)
			}
		}
	}
	if len(bootErrors) > 0 {
		in := out
		out = make(chan error, len(bootErrors))
		for _, err := range bootErrors {
			out <- err
		}
		go func(out chan<- error, in <-chan error) {
			defer close(out)
			for err := range in {
				out <- err
			}
		}(out, in)
	}

	return out
}

func TerminateOnSignal(signals ...os.Signal) Runner {
	return func(ctx context.Context) <-chan error {
		out := make(chan error)
		c := make(chan os.Signal)
		signal.Notify(c, signals...)
		go func() {
			defer close(out)
			defer close(c)
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
