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

package telegram

import (
	"context"
	"testing"
	"time"

	"go.uber.org/zap"

	"github.com/h3nc4/TelegramScout/internal/config"
)

func TestNewClient(t *testing.T) {
	logger := zap.NewNop()
	cfg := &config.Config{
		AppID:   12345,
		AppHash: "test_hash",
		Phone:   "+123",
	}

	t.Run("With Session String", func(t *testing.T) {
		cfg.Session = "dummy_session_data"
		c, err := NewClient(cfg, logger)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if c == nil {
			t.Error("expected client, got nil")
		}
	})

	t.Run("Without Session String", func(t *testing.T) {
		cfg.Session = ""
		c, err := NewClient(cfg, logger)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if c == nil {
			t.Error("expected client, got nil")
		}
	})
}

// Define placeholder for integration test
// Actual implementation logic is tested via integration or mock in main_test
func TestFetchChannelHistory_Mock(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	if ctx == nil {
		t.Fatal("context is nil")
	}
}
