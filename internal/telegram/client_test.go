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

	"github.com/gotd/td/tg"
	"go.uber.org/zap"

	"github.com/h3nc4/TelegramScout/internal/config"
	"github.com/h3nc4/TelegramScout/internal/model"
)

func TestNewClient(t *testing.T) {
	logger := zap.NewNop()
	cfg := &config.Config{
		AppID:   12345,
		AppHash: "test_hash",
		Phone:   "+123",
	}
	msgChan := make(chan model.Message)

	t.Run("With Session String", func(t *testing.T) {
		cfg.Session = "dummy_session_data"
		c, err := NewClient(cfg, logger, msgChan)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if c == nil {
			t.Error("expected client, got nil")
		}
	})
}

// Test message emission logic locally without full MTProto connection
func TestEmitMessage(t *testing.T) {
	msgChan := make(chan model.Message, 1)

	// Pre-populate peerCache
	cache := make(map[int64]peerInfo)
	cache[999] = peerInfo{
		Title:    "Test Channel",
		Username: "testchan",
	}

	client := &Client{
		msgChan:   msgChan,
		peerCache: cache,
	}

	ctx := context.Background()
	timestamp := int(time.Now().Unix())

	tgMsg := &tg.Message{
		ID:      100,
		Message: "Hello",
		Date:    timestamp,
		PeerID: &tg.PeerChannel{
			ChannelID: 999,
		},
	}

	entities := tg.Entities{
		Channels: map[int64]*tg.Channel{
			999: {
				ID:       999,
				Title:    "Test Channel",
				Username: "testchan",
			},
		},
	}

	if err := client.emitMessage(ctx, tgMsg, entities); err != nil {
		t.Fatalf("emitMessage failed: %v", err)
	}

	select {
	case m := <-msgChan:
		if m.Text != "Hello" {
			t.Errorf("expected text 'Hello', got %s", m.Text)
		}
		if m.ChatTitle != "Test Channel" {
			t.Errorf("expected title 'Test Channel', got %s", m.ChatTitle)
		}
		if m.Link != "https://t.me/testchan/100" {
			t.Errorf("unexpected link: %s", m.Link)
		}
	case <-time.After(1 * time.Second):
		t.Fatal("timeout waiting for message")
	}
}
