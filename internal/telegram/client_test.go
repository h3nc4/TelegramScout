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
	"os"
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

	// With no session in the environment the client falls back to a file, which
	// is created on the first authentication rather than here.
	t.Run("Without Session String", func(t *testing.T) {
		bare := &config.Config{AppID: 12345, AppHash: "test_hash", Phone: "+123"}
		c, err := NewClient(bare, logger, msgChan)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if c == nil {
			t.Fatal("expected client, got nil")
		}
		if _, err := os.Stat("session.json"); err == nil {
			t.Error("constructing a client wrote session.json")
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

// The peer cache is the allowlist, so every branch of it decides whether a
// message leaves the process at all.
func TestEmitMessagePeers(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name     string
		msg      *tg.Message
		entities tg.Entities
		emitted  bool
		title    string
		username string
		link     string
	}{
		{
			name:     "Channel Falls Back To Cache",
			msg:      &tg.Message{ID: 7, PeerID: &tg.PeerChannel{ChannelID: 999}},
			emitted:  true,
			title:    "Cached Channel",
			username: "cached",
			link:     "https://t.me/cached/7",
		},
		{
			name: "Chat Takes Its Title From Entities",
			msg:  &tg.Message{ID: 8, PeerID: &tg.PeerChat{ChatID: 888}},
			entities: tg.Entities{
				Chats: map[int64]*tg.Chat{888: {ID: 888, Title: "Group Chat"}},
			},
			emitted: true,
			title:   "Group Chat",
			link:    "https://t.me/c/888/8",
		},
		{
			name:    "User Has No Username So The Link Is Private",
			msg:     &tg.Message{ID: 9, PeerID: &tg.PeerUser{UserID: 777}},
			emitted: true,
			title:   "Cached User",
			link:    "https://t.me/c/777/9",
		},
		{
			name:    "Unmonitored Chat Is Dropped",
			msg:     &tg.Message{ID: 10, PeerID: &tg.PeerChannel{ChannelID: 4}},
			emitted: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			msgChan := make(chan model.Message, 1)
			client := &Client{
				msgChan: msgChan,
				peerCache: map[int64]peerInfo{
					999: {Title: "Cached Channel", Username: "cached"},
					888: {Title: "Cached Group"},
					777: {Title: "Cached User"},
				},
			}

			if err := client.emitMessage(ctx, tt.msg, tt.entities); err != nil {
				t.Fatalf("emitMessage failed: %v", err)
			}

			select {
			case m := <-msgChan:
				if !tt.emitted {
					t.Fatalf("expected no message, got %+v", m)
				}
				if m.ChatTitle != tt.title {
					t.Errorf("title = %q, want %q", m.ChatTitle, tt.title)
				}
				if m.Username != tt.username {
					t.Errorf("username = %q, want %q", m.Username, tt.username)
				}
				if m.Link != tt.link {
					t.Errorf("link = %q, want %q", m.Link, tt.link)
				}
			default:
				if tt.emitted {
					t.Error("expected a message, channel was empty")
				}
			}
		})
	}
}

func TestHandlers(t *testing.T) {
	ctx := context.Background()
	newClient := func(ch chan model.Message) *Client {
		return &Client{
			msgChan:   ch,
			peerCache: map[int64]peerInfo{5: {Title: "Monitored", Username: "monitored"}},
		}
	}
	msg := &tg.Message{ID: 1, Message: "text", PeerID: &tg.PeerChannel{ChannelID: 5}}

	t.Run("Channel Message", func(t *testing.T) {
		ch := make(chan model.Message, 1)
		u := &tg.UpdateNewChannelMessage{Message: msg}
		if err := newClient(ch).handleNewChannelMessage(ctx, tg.Entities{}, u); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(ch) != 1 {
			t.Error("expected the message to be emitted")
		}
	})

	t.Run("Message", func(t *testing.T) {
		ch := make(chan model.Message, 1)
		u := &tg.UpdateNewMessage{Message: msg}
		if err := newClient(ch).handleNewMessage(ctx, tg.Entities{}, u); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(ch) != 1 {
			t.Error("expected the message to be emitted")
		}
	})

	// A service update carries a MessageService or MessageEmpty instead, and
	// those have no text to match against.
	t.Run("Channel Non Message Is Ignored", func(t *testing.T) {
		ch := make(chan model.Message, 1)
		u := &tg.UpdateNewChannelMessage{Message: &tg.MessageEmpty{ID: 1}}
		if err := newClient(ch).handleNewChannelMessage(ctx, tg.Entities{}, u); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(ch) != 0 {
			t.Error("expected nothing to be emitted")
		}
	})

	t.Run("Non Message Is Ignored", func(t *testing.T) {
		ch := make(chan model.Message, 1)
		u := &tg.UpdateNewMessage{Message: &tg.MessageService{ID: 1}}
		if err := newClient(ch).handleNewMessage(ctx, tg.Entities{}, u); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(ch) != 0 {
			t.Error("expected nothing to be emitted")
		}
	})
}
