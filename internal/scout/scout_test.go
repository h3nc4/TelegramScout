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
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"

	"github.com/h3nc4/TelegramScout/internal/config"
	"github.com/h3nc4/TelegramScout/internal/model"
)

type MockNotifier struct {
	mu           sync.Mutex
	SentMessages []string
	NotifyChan   chan string
	Err          error
}

func (m *MockNotifier) Send(ctx context.Context, message string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.SentMessages = append(m.SentMessages, message)
	if m.NotifyChan != nil {
		m.NotifyChan <- message
	}
	return m.Err
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

func TestScout_Start(t *testing.T) {
	cfg := &config.Config{
		Monitoring: config.MonitoringRules{Keywords: []string{"urgent"}},
	}
	notifier := &MockNotifier{NotifyChan: make(chan string, 1)}
	s := New(cfg, notifier, zap.NewNop())

	ctx, cancel := context.WithCancel(context.Background())
	input := make(chan model.Message, 1)
	stopped := make(chan struct{})
	go func() {
		s.Start(ctx, input)
		close(stopped)
	}()

	input <- model.Message{ID: 1, ChatID: 2, Text: "urgent update", Date: time.Now()}
	select {
	case <-notifier.NotifyChan:
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for the loop to process a message")
	}

	cancel()
	select {
	case <-stopped:
	case <-time.After(time.Second):
		t.Fatal("Start did not return after the context was cancelled")
	}
}

func TestScout_SweepExpired(t *testing.T) {
	s := New(&config.Config{}, &MockNotifier{}, zap.NewNop())
	now := time.Now()
	s.seenMsgs.Store("2:1", now.Add(-time.Minute))
	s.seenMsgs.Store("2:2", now.Add(time.Hour))

	s.sweepExpired(now)

	if _, found := s.seenMsgs.Load("2:1"); found {
		t.Error("expected the expired entry to be dropped")
	}
	if _, found := s.seenMsgs.Load("2:2"); !found {
		t.Error("expected the live entry to be kept")
	}
}

func TestScout_CompileRulesRejectsBadRegex(t *testing.T) {
	cfg := &config.Config{
		Monitoring: config.MonitoringRules{Keywords: []string{"re:[", "urgent"}},
	}
	s := New(cfg, &MockNotifier{}, zap.NewNop())

	if len(s.rules) != 1 {
		t.Fatalf("compiled %d rules, want 1", len(s.rules))
	}
	if s.rules[0].original != "urgent" {
		t.Errorf("kept %q, want the rule that compiles", s.rules[0].original)
	}
}

// Wait for a log line rather than for a duration, so the assertion holds on a
// loaded runner.
func awaitLog(t *testing.T, logs *observer.ObservedLogs, message string) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if len(logs.FilterMessage(message).All()) > 0 {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatalf("log line %q never arrived", message)
}

func TestScout_ProcessLogsAFailedSend(t *testing.T) {
	core, logs := observer.New(zap.ErrorLevel)
	cfg := &config.Config{
		Monitoring: config.MonitoringRules{Keywords: []string{"urgent"}},
	}
	notifier := &MockNotifier{Err: errors.New("telegram refused")}
	s := New(cfg, notifier, zap.New(core))

	s.process(context.Background(), model.Message{
		ID: 1, ChatID: 2, Text: "urgent", Date: time.Now(),
	})

	awaitLog(t, logs, "Failed to send notification")
}

// The semaphore caps concurrent notifications, and a full one must not drop an
// alert. The fallback blocks until a slot frees instead.
func TestScout_ProcessWithFullSemaphore(t *testing.T) {
	core, logs := observer.New(zap.WarnLevel)
	cfg := &config.Config{
		Monitoring: config.MonitoringRules{Keywords: []string{"urgent"}},
	}
	notifier := &MockNotifier{Err: errors.New("telegram refused")}
	s := New(cfg, notifier, zap.New(core))

	// Occupy every slot, so process meets the queue as full.
	for range cap(s.notifySem) {
		s.notifySem <- struct{}{}
	}

	done := make(chan struct{})
	go func() {
		s.process(context.Background(), model.Message{
			ID: 1, ChatID: 2, Text: "urgent", Date: time.Now(),
		})
		close(done)
	}()

	// The warning proves the queue was met as full and the fallback is now
	// waiting, so freeing a slot cannot race ahead of that decision.
	awaitLog(t, logs, "Notification queue full, blocking momentarily to dispatch alert")
	<-s.notifySem

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("process did not return")
	}
	awaitLog(t, logs, "Failed to send notification")

	if got := notifier.Messages(); len(got) != 1 {
		t.Errorf("sent %d alerts, want 1", len(got))
	}
}

func TestScout_ProcessGivesUpOnCancelledContext(t *testing.T) {
	cfg := &config.Config{
		Monitoring: config.MonitoringRules{Keywords: []string{"urgent"}},
	}
	notifier := &MockNotifier{NotifyChan: make(chan string, 1)}
	s := New(cfg, notifier, zap.NewNop())

	for range cap(s.notifySem) {
		s.notifySem <- struct{}{}
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	s.process(ctx, model.Message{ID: 1, ChatID: 2, Text: "urgent", Date: time.Now()})

	select {
	case msg := <-notifier.NotifyChan:
		t.Errorf("expected nothing to be sent, got %q", msg)
	default:
	}
}

func TestScout_CleanupCacheSweeps(t *testing.T) {
	s := New(&config.Config{}, &MockNotifier{}, zap.NewNop())
	s.cleanupEvery = time.Millisecond
	s.seenMsgs.Store("2:1", time.Now().Add(-time.Hour))

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go s.cleanupCache(ctx)

	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		if _, found := s.seenMsgs.Load("2:1"); !found {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Error("the ticker never swept the expired entry")
}

func TestTruncate(t *testing.T) {
	tests := []struct {
		name string
		in   string
		max  int
		want string
	}{
		{"Under The Limit", "short", 10, "short"},
		{"At The Limit", "exactly10!", 10, "exactly10!"},
		{"Over The Limit", "abcdefghijk", 10, "abcdefghij..."},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := truncate(tt.in, tt.max); got != tt.want {
				t.Errorf("truncate(%q, %d) = %q, want %q", tt.in, tt.max, got, tt.want)
			}
		})
	}
}
