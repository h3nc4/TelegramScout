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

package logger

import (
	"testing"

	"go.uber.org/zap"
)

func TestNew(t *testing.T) {
	l, err := New()
	if err != nil {
		t.Fatalf("failed to create logger: %v", err)
	}
	if l == nil {
		t.Fatal("expected logger instance, got nil")
	}

	// Verify logging to both streams without panic
	l.Info("test info message")
	l.Warn("test warning message")
	l.Error("test error message")

	// Verify structured logging fields
	l.Info("test with fields", zap.String("key", "value"))

	// Ignore sync error on stdout/stderr
	_ = l.Sync()
}
