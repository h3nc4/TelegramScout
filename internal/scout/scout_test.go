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

package scout

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"

	"go.uber.org/zap"

	"github.com/h3nc4/TelegramScout/internal/config"
	"github.com/h3nc4/TelegramScout/internal/model"
)

type MockNotifier struct {
	mu           sync.Mutex
	SentMessages []string
	NotifyChan   chan string
}

func (m *MockNotifier) Send(ctx context.Context, message string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.SentMessages = append(m.SentMessages, message)
	if m.NotifyChan != nil {
		m.NotifyChan <- message
	}
	return nil
}

func (m *MockNotifier) Messages() []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	// Return copy
	return append([]string(nil), m.SentMessages...)
}

func TestScout_Process(t *testing.T) {
	log := zap.NewNop()
	cfg := &config.Config{
		Monitoring: config.MonitoringRules{
			Keywords: []string{
				"bitcoin",
				"urgent",
				"rtx * 5070",    // Glob
				"hello world",   // Simple with space
				"re:(?i)b[oa]t", // Regex
			},
		},
	}
	notifier := &MockNotifier{
		NotifyChan: make(chan string, 10),
	}
	s := New(cfg, notifier, log)

	tests := []struct {
		name        string
		text        string
		shouldMatch bool
		keyword     string
	}{
		{"Simple Match", "Bitcoin is soaring", true, "bitcoin"},
		{"Case Insensitive", "BITCOIN is up", true, "bitcoin"},
		{"No Match", "Ethereum is down", false, ""},
		{"Glob Match", "Selling RTX Super 5070 cheap", true, "rtx * 5070"},
		{"Glob Multiline", "RTX\nSuper 5070", true, "rtx * 5070"},
		{"Glob Fail", "RTX 4070", false, ""},
		{"Simple Multiline", "Hello\nWorld", true, "hello world"},
		{"Simple Extra Spaces", "Hello    World", true, "hello world"},
		{"Simple Normal", "Hello World", true, "hello world"},
		{"Regex Match 'bat'", "I saw a bat", true, "re:(?i)b[oa]t"},
		{"Regex Match 'bot'", "I saw a bot", true, "re:(?i)b[oa]t"},
		{"Regex Fail", "I saw a bit", false, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Clear previous
			notifier.mu.Lock()
			notifier.SentMessages = nil
			notifier.mu.Unlock()

			msg := model.Message{
				ID:        1,
				ChatID:    100,
				ChatTitle: "Test",
				Text:      tt.text,
				Date:      time.Now(),
				Link:      "http://t.me/msg/1",
			}
			// Bypass dedup for testing by ensuring unique ID effectively (or clearing map)
			s.seenMsgs = sync.Map{}

			s.process(context.Background(), msg)

			if tt.shouldMatch {
				select {
				case received := <-notifier.NotifyChan:
					if !strings.Contains(received, tt.keyword) {
						t.Errorf("expected keyword %q in alert, got message: %s", tt.keyword, received)
					}
				case <-time.After(100 * time.Millisecond):
					t.Errorf("timeout waiting for notification for text %q", tt.text)
				}
			} else {
				select {
				case <-notifier.NotifyChan:
					t.Errorf("expected no match for text %q, but got notification", tt.text)
				case <-time.After(50 * time.Millisecond):
					// No message received, pass
				}
			}
		})
	}

	t.Run("Deduplication", func(t *testing.T) {
		notifier.mu.Lock()
		notifier.SentMessages = nil
		notifier.mu.Unlock()
		s.seenMsgs = sync.Map{}
		msg := model.Message{
			ID:     999,
			ChatID: 100,
			Text:   "urgent update",
			Date:   time.Now(),
		}

		// First pass
		s.process(context.Background(), msg)
		select {
		case <-notifier.NotifyChan:
			// OK
		case <-time.After(100 * time.Millisecond):
			t.Fatal("expected notification on first pass")
		}

		// Second pass (duplicate)
		s.process(context.Background(), msg)
		select {
		case <-notifier.NotifyChan:
			t.Fatal("expected no new notification on duplicate pass")
		case <-time.After(50 * time.Millisecond):
			// OK
		}
	})
}
