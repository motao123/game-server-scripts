package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"game-server-scripts/internal/app"
	"game-server-scripts/internal/config"
	"game-server-scripts/internal/install"
	"game-server-scripts/internal/palworld"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		log.Fatalf("error: %v", err)
	}
}

func run(args []string) error {
	if len(args) == 0 {
		args = []string{"web"}
	}

	switch args[0] {
	case "web":
		cfg := config.Load()
		srv, err := app.NewServer(cfg)
		if err != nil {
			return err
		}
		ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
		defer stop()
		return srv.Run(ctx)
	case "install":
		return install.Run(args[1:])
	case "palworld":
		return palworld.RunCLI(args[1:])
	case "version":
		fmt.Println("gsm-panel 0.1.0")
		return nil
	default:
		return fmt.Errorf("unknown command %q", args[0])
	}
}
