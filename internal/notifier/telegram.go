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
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"go.uber.org/zap"

	"github.com/h3nc4/TelegramScout/internal/config"
)

// Define interface for sending alerts
type Notifier interface {
	Send(ctx context.Context, message string) error
}

// Send messages using the Telegram Bot API
type TelegramNotifier struct {
	client  *http.Client
	log     *zap.Logger
	token   string
	chatID  int64
	baseURL string
}

// Create new TelegramNotifier
func New(cfg *config.Config, log *zap.Logger) *TelegramNotifier {
	return &TelegramNotifier{
		client: &http.Client{
			Timeout: 15 * time.Second,
		},
		log:     log,
		token:   cfg.BotToken,
		chatID:  cfg.ChatID,
		baseURL: "https://api.telegram.org",
	}
}

// Post text message to configured chat
func (t *TelegramNotifier) Send(ctx context.Context, message string) error {
	url := fmt.Sprintf("%s/bot%s/sendMessage", t.baseURL, t.token)

	payload := map[string]interface{}{
		"chat_id":                  t.chatID,
		"text":                     message,
		"parse_mode":               "HTML",
		"disable_web_page_preview": true,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal payload: %w", err)
	}

	const maxRetries = 3
	var lastErr error

	for i := range maxRetries {
		if ctx.Err() != nil {
			return ctx.Err()
		}

		err := t.attemptSend(ctx, url, body)
		if err == nil {
			t.log.Info("Notification sent", zap.Int64("chat_id", t.chatID))
			return nil
		}

		lastErr = err
		t.log.Warn("Failed to send notification, retrying...",
			zap.Int("attempt", i+1),
			zap.Error(err),
		)

		// Exponential backoff
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(time.Duration(1<<i) * time.Second):
			continue
		}
	}

	return fmt.Errorf("failed to send notification after %d attempts: %w", maxRetries, lastErr)
}

func (t *TelegramNotifier) attemptSend(ctx context.Context, url string, body []byte) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := t.client.Do(req)
	if err != nil {
		return fmt.Errorf("network error: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == http.StatusOK {
		return nil
	}

	// Handle Rate Limiting
	if resp.StatusCode == http.StatusTooManyRequests {
		retryAfterStr := resp.Header.Get("Retry-After")
		retryAfter, _ := strconv.Atoi(retryAfterStr)
		if retryAfter == 0 {
			retryAfter = 5 // Default backoff
		}
		return fmt.Errorf("rate limited, retry after %d seconds", retryAfter)
	}

	return fmt.Errorf("api returned status: %d", resp.StatusCode)
}
