package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/caretex/caretexnursing.core/internal/config"
	"github.com/caretex/caretexnursing.core/internal/jobs"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

func main() {
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnix
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr, TimeFormat: time.RFC3339})

	cfg, err := config.Load()
	if err != nil {
		log.Fatal().Err(err).Msg("failed to load config")
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	runner := jobs.NewRunner(cfg)
	go runner.Start(ctx)

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Info().Msg("worker shutting down")
	cancel()
	time.Sleep(2 * time.Second)
}
