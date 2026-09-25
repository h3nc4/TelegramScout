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
	"sync"

	"github.com/gotd/td/session"
)

// Implement session.Storage for in-memory handling
type memorySession struct {
	data []byte
	mux  sync.RWMutex
}

// Retrieve session data from memory
func (m *memorySession) LoadSession(ctx context.Context) ([]byte, error) {
	m.mux.RLock()
	defer m.mux.RUnlock()
	if len(m.data) == 0 {
		return nil, session.ErrNotFound
	}
	// Return a copy to ensure thread safety
	out := make([]byte, len(m.data))
	copy(out, m.data)
	return out, nil
}

// Store session data in memory
func (m *memorySession) StoreSession(ctx context.Context, data []byte) error {
	m.mux.Lock()
	defer m.mux.Unlock()
	m.data = make([]byte, len(data))
	copy(m.data, data)
	return nil
}
