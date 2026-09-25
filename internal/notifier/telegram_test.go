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

package notifier

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"go.uber.org/zap"

	"github.com/h3nc4/TelegramScout/internal/config"
)

func TestTelegramNotifier_Send(t *testing.T) {
	log := zap.NewNop()
	cfg := &config.Config{
		BotToken: "test_token",
		ChatID:   123456,
	}

	t.Run("Success with HTML formatting", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodPost {
				t.Errorf("expected POST method, got %s", r.Method)
			}
			if r.URL.Path != "/bottest_token/sendMessage" {
				t.Errorf("unexpected path: %s", r.URL.Path)
			}

			// Verify payload contains parse_mode
			var payload map[string]interface{}
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				t.Errorf("failed to decode body: %v", err)
			}

			if payload["parse_mode"] != "HTML" {
				t.Errorf("expected parse_mode HTML, got %v", payload["parse_mode"])
			}
			if payload["text"] != "<b>Hello</b>" {
				t.Errorf("expected text <b>Hello</b>, got %v", payload["text"])
			}

			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()

		n := New(cfg, log)
		n.baseURL = server.URL // Override base URL for testing

		if err := n.Send(context.Background(), "<b>Hello</b>"); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("Retry on 500", func(t *testing.T) {
		var calls int32
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			count := atomic.AddInt32(&calls, 1)
			if count < 3 {
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()

		n := New(cfg, log)
		n.baseURL = server.URL
		n.backoff = time.Millisecond

		if err := n.Send(context.Background(), "RetryMe"); err != nil {
			t.Errorf("expected success after retry, got error: %v", err)
		}
		if atomic.LoadInt32(&calls) != 3 {
			t.Errorf("expected 3 calls, got %d", calls)
		}
	})

	t.Run("Fail after max retries", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
		}))
		defer server.Close()

		n := New(cfg, log)
		n.baseURL = server.URL
		n.backoff = time.Millisecond

		if err := n.Send(context.Background(), "FailMe"); err == nil {
			t.Error("expected error after max retries, got nil")
		}
	})

	// Telegram answers 429 with the wait in a header, and the API is free to
	// leave it out, in which case the message reports the fallback.
	t.Run("Rate limited with a Retry-After header", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Retry-After", "3")
			w.WriteHeader(http.StatusTooManyRequests)
		}))
		defer server.Close()

		n := New(cfg, log)
		n.baseURL = server.URL
		n.backoff = time.Millisecond

		err := n.Send(context.Background(), "TooFast")
		if err == nil {
			t.Fatal("expected an error, got nil")
		}
		if !strings.Contains(err.Error(), "retry after 3 seconds") {
			t.Errorf("expected the header value in the error, got %v", err)
		}
	})

	t.Run("Rate limited without a Retry-After header", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusTooManyRequests)
		}))
		defer server.Close()

		n := New(cfg, log)
		n.baseURL = server.URL
		n.backoff = time.Millisecond

		err := n.Send(context.Background(), "TooFast")
		if err == nil {
			t.Fatal("expected an error, got nil")
		}
		if !strings.Contains(err.Error(), "retry after 5 seconds") {
			t.Errorf("expected the fallback wait in the error, got %v", err)
		}
	})

	t.Run("Network error", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
		url := server.URL
		// Closed before the send, so the port answers nothing.
		server.Close()

		n := New(cfg, log)
		n.baseURL = url
		n.backoff = time.Millisecond

		err := n.Send(context.Background(), "Nobody home")
		if err == nil {
			t.Fatal("expected an error, got nil")
		}
		if !strings.Contains(err.Error(), "network error") {
			t.Errorf("expected a network error, got %v", err)
		}
	})

	t.Run("Unusable URL", func(t *testing.T) {
		n := New(cfg, log)
		// A control character, which url.Parse refuses.
		n.baseURL = "http://127.0.0.1\x7f"
		n.backoff = time.Millisecond

		err := n.Send(context.Background(), "Nowhere")
		if err == nil {
			t.Fatal("expected an error, got nil")
		}
		if !strings.Contains(err.Error(), "failed to create request") {
			t.Errorf("expected a request construction error, got %v", err)
		}
	})

	t.Run("Context cancelled before the first attempt", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			t.Error("expected no request once the context is cancelled")
		}))
		defer server.Close()

		n := New(cfg, log)
		n.baseURL = server.URL

		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		if err := n.Send(ctx, "Too late"); !errors.Is(err, context.Canceled) {
			t.Errorf("expected context.Canceled, got %v", err)
		}
	})

	t.Run("Context cancelled during the backoff", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// The attempt fails and the context is gone before the wait ends.
			cancel()
			w.WriteHeader(http.StatusInternalServerError)
		}))
		defer server.Close()

		n := New(cfg, log)
		n.baseURL = server.URL
		n.backoff = 10 * time.Second

		if err := n.Send(ctx, "Give up"); !errors.Is(err, context.Canceled) {
			t.Errorf("expected context.Canceled, got %v", err)
		}
	})
}
