package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/AbdeljalilB92/lldl/app"
	"github.com/AbdeljalilB92/lldl/shared/logging"
)

const banner = `
╔═╗╔═╗────╔╗───╔╗──────────╔╗─────────╔╗──────────────╔╗─╔╗
║║╚╝║║────║║───║║──────────║║─────────║║──────────────║║─║║
║╔╗╔╗╠══╦═╝╠══╗║╚═╦╗─╔╦╗╔══╣╚═╦╗╔╦══╦═╣╚═╗╔╗─╔╗╔╦══╦═╬╬═╝╠══╗
║║║║║║╔╗║╔╗║║═╣║╔╗║║─║╠╝║╔╗║╔╗║╚╝║║═╣╔╗║╔╗║║─║║╚╝║╔╗║╔╬╦═╗║╔╗║
║║║║║║╔╗║╚╝║║═╣║╚╝║╚═╝╠╗║╔╗║║║║║║║║═╣╚╝║╔╗║╚═╝║║║║╔╗║║║║─║║╔╗║
╚╝╚╝╚╩╝╚╩══╩══╝╚══╩═╗╔╩╝╚╝╚╩╝╚╩╩╩╩══╩══╩╝╚╩═╗╔╩╩╩╩╝╚╩╝╚╝─╚╩╝╚╝
──────────────────╔═╝║────────────────────╔═╝║
──────────────────╚══╝────────────────────╚══╝
`

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	cleanup, err := logging.Setup(logging.DefaultLevel(), "./logs")
	if err != nil {
		slog.Warn("logging setup failed, continuing with default logger", "error", err)
	} else {
		defer cleanup()
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	fmt.Print(banner)

	application, err := app.Wire(app.WireConfig{})
	if err != nil {
		return fmt.Errorf("application wiring failed: %w", err)
	}

	return application.Run(ctx)
}
