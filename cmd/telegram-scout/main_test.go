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
	"bytes"
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/gotd/td/tg"
	"go.uber.org/zap"

	"github.com/h3nc4/TelegramScout/internal/config"
)

type mockClient struct {
	runFunc func(ctx context.Context, handler func(ctx context.Context, api *tg.Client) error) error
}

func (m *mockClient) Run(ctx context.Context, handler func(ctx context.Context, api *tg.Client) error) error {
	if m.runFunc != nil {
		return m.runFunc(ctx, handler)
	}
	return nil
}

type mockNotifier struct {
	sendFunc func(ctx context.Context, message string) error
}

func (m *mockNotifier) Send(ctx context.Context, message string) error {
	if m.sendFunc != nil {
		return m.sendFunc(ctx, message)
	}
	return nil
}

func TestRun(t *testing.T) {
	log := zap.NewNop()
	cfg := &config.Config{
		AppID:         12345,
		TargetChannel: "test_channel",
		Limit:         10,
	}

	noopNotifier := &mockNotifier{}

	t.Run("Successful Run", func(t *testing.T) {
		var buf bytes.Buffer
		client := &mockClient{
			runFunc: func(ctx context.Context, handler func(ctx context.Context, api *tg.Client) error) error {
				_, _ = fmt.Fprintf(&buf, "\n--- Messages from %s ---\n", cfg.TargetChannel)
				_, _ = fmt.Fprintln(&buf, "[1234567890] Hello World")
				_, _ = fmt.Fprintln(&buf, "--- End of fetch ---")
				return nil
			},
		}

		app := &AppContext{
			Log:      log,
			Config:   cfg,
			Client:   client,
			Notifier: noopNotifier,
			Writer:   &buf,
		}

		if err := run(context.Background(), app); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		output := buf.String()
		if output == "" {
			t.Error("expected output, got empty string")
		}
	})

	t.Run("Notifier Error Should Not Fatal", func(t *testing.T) {
		var buf bytes.Buffer
		client := &mockClient{} // No-op success
		failNotifier := &mockNotifier{
			sendFunc: func(ctx context.Context, message string) error {
				return errors.New("API down")
			},
		}

		app := &AppContext{
			Log:      log,
			Config:   cfg,
			Client:   client,
			Notifier: failNotifier,
			Writer:   &buf,
		}

		if err := run(context.Background(), app); err != nil {
			t.Errorf("expected no error despite notifier fail, got: %v", err)
		}
	})

	t.Run("Client Error", func(t *testing.T) {
		client := &mockClient{
			runFunc: func(ctx context.Context, handler func(ctx context.Context, api *tg.Client) error) error {
				return errors.New("connection failed")
			},
		}

		app := &AppContext{
			Log:      log,
			Config:   cfg,
			Client:   client,
			Notifier: noopNotifier,
			Writer:   &bytes.Buffer{},
		}

		err := run(context.Background(), app)
		if err == nil {
			t.Error("expected error, got nil")
		}
	})
}
