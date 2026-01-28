/*
 * Copyright (C) 2026  Henrique Almeida
 * This file is part of TelegramScout.
 *
 * TelegramScout is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published
 * by the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * TelegramScout is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with TelegramScout.  If not, see <https://www.gnu.org/licenses/>.
 */

package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"go.uber.org/zap"

	"github.com/h3nc4/TelegramScout/internal/config"
	"github.com/h3nc4/TelegramScout/internal/logger"
	"github.com/h3nc4/TelegramScout/internal/model"
	"github.com/h3nc4/TelegramScout/internal/notifier"
	"github.com/h3nc4/TelegramScout/internal/scout"
	"github.com/h3nc4/TelegramScout/internal/telegram"
)

func main() {
	// Initialize context
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	// Initialize logger
	log, err := logger.New()
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "failed to initialize logger: %v\n", err)
		os.Exit(1)
	}
	defer func() { _ = log.Sync() }()

	if err := run(ctx, log); err != nil {
		log.Fatal("Application startup failed", zap.Error(err))
	}
}

func run(ctx context.Context, log *zap.Logger) error {
	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load configuration: %w", err)
	}

	if len(cfg.Monitoring.Chats) == 0 {
		return fmt.Errorf("no chats configured for monitoring")
	}

	// Channel for streaming messages from Telegram client to Scout
	msgChan := make(chan model.Message, 100)

	// Initialize Notifier (Bot API)
	notif := notifier.New(cfg, log)

	// Initialize Scout
	s := scout.New(cfg, notif, log)

	// Start Scout consumer in background
	go s.Start(ctx, msgChan)

	log.Info("Starting TelegramScout",
		zap.Int("monitored_chats", len(cfg.Monitoring.Chats)),
		zap.Int("keywords", len(cfg.Monitoring.Keywords)),
	)

	// Send startup notification
	if err := notif.Send(ctx, "TelegramScout is now online and monitoring."); err != nil {
		log.Error("failed to send startup notification", zap.Error(err))
	}

	// Enter supervisor loop
	runSupervisor(ctx, cfg, log, msgChan)

	log.Info("TelegramScout shutdown complete")
	return nil
}

func runSupervisor(ctx context.Context, cfg *config.Config, log *zap.Logger, msgChan chan<- model.Message) {
	backoff := time.Second
	maxBackoff := 1 * time.Minute

	for {
		// Check context before restarting
		if ctx.Err() != nil {
			log.Info("Context cancelled, shutting down supervisor")
			return
		}

		shouldRetry, err := startClientSession(ctx, cfg, log, msgChan)
		if !shouldRetry {
			if err != nil {
				// Fatal error during initialization
				log.Fatal("failed to initialize telegram client", zap.Error(err))
			}
			// Graceful exit
			return
		}

		// Runtime error, attempt restart
		log.Error("Telegram client crashed, restarting...", zap.Error(err), zap.Duration("backoff", backoff))

		select {
		case <-ctx.Done():
			return
		case <-time.After(backoff):
			// Exponential backoff with cap
			backoff *= 2
			if backoff > maxBackoff {
				backoff = maxBackoff
			}
		}
	}
}

func startClientSession(ctx context.Context, cfg *config.Config, log *zap.Logger, msgChan chan<- model.Message) (bool, error) {
	log.Info("Initializing Telegram Client...")
	client, err := telegram.NewClient(cfg, log, msgChan)
	if err != nil {
		return false, err
	}

	// Run Telegram Client (Blocking)
	if err := client.Run(ctx); err != nil {
		// If context is canceled, it's a graceful shutdown
		if errors.Is(err, context.Canceled) {
			log.Info("Telegram client stopped (context canceled)")
			return false, nil
		}
		// Unexpected error
		return true, err
	}

	// Clean exit
	log.Info("Telegram client stopped gracefully")
	return false, nil
}
