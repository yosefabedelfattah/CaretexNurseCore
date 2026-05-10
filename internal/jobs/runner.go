// Package jobs provides background workers for outbox processing,
// scheduled Caretx pulls, and other async tasks.
//
// This file is a scaffold; flesh out the runner to consume your outbox table.
package jobs

import (
	"context"
	"time"

	"github.com/caretex/caretexnursing.core/internal/config"
	"github.com/rs/zerolog/log"
)

type Runner struct{ cfg *config.Config }

func NewRunner(cfg *config.Config) *Runner { return &Runner{cfg: cfg} }

func (r *Runner) Start(ctx context.Context) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	log.Info().Msg("worker started")
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			r.tick(ctx)
		}
	}
}

func (r *Runner) tick(ctx context.Context) {
	// TODO: drain outbox, push to Caretx, retry failed jobs, pull external changes
	log.Debug().Msg("worker tick (no-op)")
}
