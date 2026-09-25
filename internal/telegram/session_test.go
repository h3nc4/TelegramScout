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
	"bytes"
	"context"
	"errors"
	"testing"

	"github.com/gotd/td/session"
)

func TestMemorySession(t *testing.T) {
	ctx := context.Background()

	t.Run("Empty Reports Not Found", func(t *testing.T) {
		m := &memorySession{}
		if _, err := m.LoadSession(ctx); !errors.Is(err, session.ErrNotFound) {
			t.Errorf("expected session.ErrNotFound, got %v", err)
		}
	})

	t.Run("Round Trip", func(t *testing.T) {
		m := &memorySession{}
		if err := m.StoreSession(ctx, []byte("session-bytes")); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		got, err := m.LoadSession(ctx)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !bytes.Equal(got, []byte("session-bytes")) {
			t.Errorf("loaded %q, want %q", got, "session-bytes")
		}
	})

	t.Run("Load Returns A Copy", func(t *testing.T) {
		m := &memorySession{data: []byte("original")}
		got, err := m.LoadSession(ctx)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		got[0] = 'X'
		if !bytes.Equal(m.data, []byte("original")) {
			t.Errorf("stored data became %q, so the caller holds the same array", m.data)
		}
	})

	t.Run("Store Keeps Its Own Array", func(t *testing.T) {
		data := []byte("stored")
		m := &memorySession{}
		if err := m.StoreSession(ctx, data); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		data[0] = 'X'
		if !bytes.Equal(m.data, []byte("stored")) {
			t.Errorf("stored data became %q, so the writer holds the same array", m.data)
		}
	})
}
