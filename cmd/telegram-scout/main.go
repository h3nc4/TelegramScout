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
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"

	"github.com/gotd/td/tg"
	"go.uber.org/zap"

	"github.com/h3nc4/TelegramScout/internal/config"
	"github.com/h3nc4/TelegramScout/internal/logger"
	"github.com/h3nc4/TelegramScout/internal/notifier"
	"github.com/h3nc4/TelegramScout/internal/telegram"
)

// Hold dependencies for the application
type AppContext struct {
	Log      *zap.Logger
	Config   *config.Config
	Client   telegram.ScoutClient
	Notifier notifier.Notifier
	Writer   io.Writer
}

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

	// Load configuration
	cfg, err := config.LoadFromEnv()
	if err != nil {
		log.Fatal("failed to load configuration", zap.Error(err))
	}

	// Initialize Telegram client (MTProto)
	client, err := telegram.NewClient(cfg, log)
	if err != nil {
		log.Fatal("failed to initialize telegram client", zap.Error(err))
	}

	// Initialize Notifier (Bot API)
	notif := notifier.New(cfg, log)

	app := &AppContext{
		Log:      log,
		Config:   cfg,
		Client:   client,
		Notifier: notif,
		Writer:   os.Stdout,
	}

	if err := run(ctx, app); err != nil {
		log.Error("application error", zap.Error(err))
		os.Exit(1)
	}
}

// Execute main application logic
func run(ctx context.Context, app *AppContext) error {
	app.Log.Info("Starting TelegramScout",
		zap.String("channel_input", app.Config.TargetChannel),
		zap.Int("app_id", app.Config.AppID),
	)

	// Send notification via Bot
	app.Log.Info("Sending startup notification...")
	if err := app.Notifier.Send(ctx, "online"); err != nil {
		app.Log.Error("failed to send startup notification", zap.Error(err))
		// Continue despite notification failure
	}

	return app.Client.Run(ctx, func(ctx context.Context, api *tg.Client) error {
		app.Log.Info("Connected to Telegram. Resolving peer and fetching history...")

		messages, resolvedID, err := telegram.FetchChannelHistory(ctx, api, app.Config.TargetChannel, app.Config.Limit)
		if err != nil {
			return fmt.Errorf("fetching history failed: %w", err)
		}

		app.Log.Info("Resolved Peer",
			zap.String("input", app.Config.TargetChannel),
			zap.Int64("peer_id", resolvedID),
		)

		app.Log.Info("Fetched messages", zap.Int("count", len(messages)))
		fmt.Fprintf(app.Writer, "\n--- Messages from %s (ID: %d) ---\n", app.Config.TargetChannel, resolvedID)
		for _, msg := range messages {
			fmt.Fprintln(app.Writer, msg)
		}
		fmt.Fprintln(app.Writer, "--- End of fetch ---")

		return nil
	})
}
