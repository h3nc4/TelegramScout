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
	"fmt"
	"regexp"
	"strings"
	"sync"
	"time"

	"go.uber.org/zap"

	"github.com/h3nc4/TelegramScout/internal/config"
	"github.com/h3nc4/TelegramScout/internal/model"
	"github.com/h3nc4/TelegramScout/internal/notifier"
)

// Encapsulate a compiled matching strategy
type matchRule struct {
	original string
	check    func(text string) bool
}

// Process incoming messages and triggers alerts
type Scout struct {
	cfg      *config.Config
	notifier notifier.Notifier
	log      *zap.Logger

	// Compiled matching rules
	rules []matchRule

	// Dedup cache: Key = "ChatID:MsgID", Value = Expiration
	seenMsgs sync.Map

	// Semaphore to limit concurrent notification requests
	notifySem chan struct{}
}

// Create a new Scout instance and compiles matching rules
func New(cfg *config.Config, notifier notifier.Notifier, log *zap.Logger) *Scout {
	s := &Scout{
		cfg:      cfg,
		notifier: notifier,
		log:      log,
		// Limit concurrent notifications
		notifySem: make(chan struct{}, 5),
	}
	s.compileRules()
	return s
}

// Process config keywords into efficient matching functions
func (s *Scout) compileRules() {
	var rules []matchRule

	for _, k := range s.cfg.Monitoring.Keywords {
		k := k // Capture for closure
		var check func(string) bool

		switch {
		// Explicit Regex (prefix "re:")
		case strings.HasPrefix(k, "re:"):
			pattern := k[3:]
			re, err := regexp.Compile(pattern)
			if err != nil {
				s.log.Error("Invalid regex keyword ignored", zap.String("keyword", k), zap.Error(err))
				continue
			}
			check = func(text string) bool {
				return re.MatchString(text)
			}

		// Glob Pattern (contains "*")
		case strings.Contains(k, "*"):
			// Escape everything except '*', then replace '*' with '.*'
			parts := strings.Split(k, "*")
			for i := range parts {
				quoted := regexp.QuoteMeta(parts[i])
				parts[i] = strings.ReplaceAll(quoted, " ", `\s+`)
			}
			pattern := "(?si)" + strings.Join(parts, ".*")
			re := regexp.MustCompile(pattern)
			check = func(text string) bool {
				return re.MatchString(text)
			}

		// Simple Substring
		default:
			if strings.Contains(k, " ") {
				// Lenient matching for phrases with spaces
				quoted := regexp.QuoteMeta(k)
				pattern := "(?si)" + strings.ReplaceAll(quoted, " ", `\s+`)
				re := regexp.MustCompile(pattern)
				check = func(text string) bool {
					return re.MatchString(text)
				}
			} else {
				// Fast path for single words
				lowK := strings.ToLower(k)
				check = func(text string) bool {
					return strings.Contains(strings.ToLower(text), lowK)
				}
			}
		}

		rules = append(rules, matchRule{
			original: k,
			check:    check,
		})
	}

	s.rules = rules
}

// Listen to the message channel and process messages
func (s *Scout) Start(ctx context.Context, input <-chan model.Message) {
	// Start cleanup ticker for deduplication cache
	go s.cleanupCache(ctx)

	for {
		select {
		case <-ctx.Done():
			return
		case msg := <-input:
			s.process(ctx, msg)
		}
	}
}

func (s *Scout) process(ctx context.Context, msg model.Message) {
	// Check Deduplication
	dedupKey := fmt.Sprintf("%d:%d", msg.ChatID, msg.ID)
	if _, exists := s.seenMsgs.Load(dedupKey); exists {
		return
	}

	// Rule Matching
	matchedKeyword := ""
	for _, rule := range s.rules {
		if rule.check(msg.Text) {
			matchedKeyword = rule.original
			break
		}
	}

	if matchedKeyword == "" {
		return
	}

	// Mark as seen
	s.seenMsgs.Store(dedupKey, time.Now().Add(1*time.Hour))
	s.log.Info("Keyword matched",
		zap.String("keyword", matchedKeyword),
		zap.String("channel", msg.ChatTitle),
		zap.Int("msg_id", msg.ID),
	)

	// Build Alert
	alertText := fmt.Sprintf(
		"🚨 <b>Match:</b> %s\n"+
			"📢 <b>Chat:</b> %s\n"+
			"🕒 <b>Time:</b> %s\n"+
			"🔗 <a href=\"%s\">Link to Message</a>\n\n"+
			"<i>%s</i>",
		matchedKeyword,
		msg.ChatTitle,
		msg.Date.Format(time.Kitchen),
		msg.Link,
		truncate(msg.Text, 200),
	)

	// Dispatch notification asynchronously to not block the reader loop
	select {
	case s.notifySem <- struct{}{}:
		go func() {
			defer func() { <-s.notifySem }()
			if err := s.notifier.Send(ctx, alertText); err != nil {
				s.log.Error("Failed to send notification", zap.Error(err))
			}
		}()
	case <-ctx.Done():
		return
	default:
		s.log.Warn("Notification queue full, blocking momentarily to dispatch alert", zap.Int("msg_id", msg.ID))
		// Fallback to blocking send if queue is full to ensure alerts are not dropped
		s.notifySem <- struct{}{}
		go func() {
			defer func() { <-s.notifySem }()
			if err := s.notifier.Send(ctx, alertText); err != nil {
				s.log.Error("Failed to send notification", zap.Error(err))
			}
		}()
	}
}

// Remove old entries from deduplication map
func (s *Scout) cleanupCache(ctx context.Context) {
	ticker := time.NewTicker(10 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			now := time.Now()
			s.seenMsgs.Range(func(key, value interface{}) bool {
				expiry := value.(time.Time)
				if now.After(expiry) {
					s.seenMsgs.Delete(key)
				}
				return true
			})
		}
	}
}

func truncate(s string, max int) string {
	if len(s) > max {
		return s[:max] + "..."
	}
	return s
}
