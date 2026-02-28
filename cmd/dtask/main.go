package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/kristianvld/dtask/internal/app"
	"github.com/kristianvld/dtask/internal/version"
)

func main() {
	showVersion := flag.Bool("version", false, "print version and exit")
	flag.Parse()

	if *showVersion {
		fmt.Println(version.String())
		return
	}

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	if err := app.Run(ctx, os.Environ()); err != nil {
		fmt.Fprintf(os.Stderr, "dtask: %v\n", err)
		os.Exit(1)
	}
}
