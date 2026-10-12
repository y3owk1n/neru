package client_test

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/y3owk1n/neru/client"
)

//nolint:testableexamples // it needs a running daemon, so it has no fixed output.
func Example() {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	neru := client.New()

	status, err := neru.Status(ctx)
	if errors.Is(err, client.ErrNoDaemon) {
		fmt.Println("launch neru first")

		return
	}

	if err != nil {
		fmt.Println(err)

		return
	}

	fmt.Println("mode:", status.Mode)

	// Every command without a typed helper goes through Send.
	_, err = neru.Send(ctx, "hints", "--action", "left_click")

	if refusal, ok := errors.AsType[*client.Error](err); ok {
		fmt.Println("neru refused:", refusal.Code)
	}
}

//nolint:testableexamples // it needs a running daemon, so it has no fixed output.
func ExampleClient_Watch() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err := client.New().Watch(ctx, func(line client.WatchLine) error {
		if line.Event == "mode_enter" {
			fmt.Println("entered", line.Mode)
		}

		return nil
	})
	if err != nil {
		fmt.Println(err)
	}
}
