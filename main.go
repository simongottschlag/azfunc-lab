package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/KarlGW/azfunc"
	"github.com/KarlGW/azfunc/triggers"
	"golang.org/x/sync/errgroup"
)

func main() {
	err := run(context.Background())
	if err != nil {
		fmt.Fprintf(os.Stderr, "application returned an error: %v\n", err)
		os.Exit(1)
	}
}

func run(ctx context.Context) error {
	ctx, stop := signal.NotifyContext(ctx, syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	app := azfunc.NewFunctionApp()

	app.AddFunction("stage1", azfunc.HTTPTrigger(func(ctx *azfunc.Context, trigger *triggers.HTTP) {
		var input struct {
			Message string `json:"message"`
		}
		if err := trigger.Parse(&input); err != nil {
			ctx.Output.HTTP().WriteHeader(http.StatusBadRequest)
			return
		}

		ctx.Output.HTTP().WriteHeader(http.StatusOK)
		ctx.Output.HTTP().Header().Add("Content-Type", "application/json")
		ctx.Output.HTTP().Write([]byte(fmt.Sprintf(`{"input_message":"%s"}`, input.Message)))

		ctx.Output.Binding("servicebus").Write([]byte(fmt.Sprintf(`{"input_message":"%s"}`, input.Message)))
	}))

	app.AddFunction("stage2", azfunc.QueueTrigger("servicebus", func(ctx *azfunc.Context, trigger *triggers.Queue) {
		var input struct {
			Message string `json:"input_message"`
		}
		if err := trigger.Parse(&input); err != nil {
			ctx.SetError(err)
			return
		}

		ctx.Log().Info("queue message received", "input_message", input.Message)
	}))

	g, _ := errgroup.WithContext(ctx)

	g.Go(func() error {
		return app.Start()
	})

	return g.Wait()
}
