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
	"strings"
	"testing"

	"github.com/gotd/td/tg"
)

func TestTerminalAuthenticator(t *testing.T) {
	ctx := context.Background()

	t.Run("Phone", func(t *testing.T) {
		auth := &terminalAuthenticator{phone: "+123456"}
		phone, err := auth.Phone(ctx)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if phone != "+123456" {
			t.Errorf("expected phone +123456, got %s", phone)
		}
	})

	t.Run("Password from Config", func(t *testing.T) {
		auth := &terminalAuthenticator{password: "secret"}
		pwd, err := auth.Password(ctx)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if pwd != "secret" {
			t.Errorf("expected password 'secret', got %s", pwd)
		}
	})

	t.Run("Password Interactive", func(t *testing.T) {
		input := "interactive_pass\n"
		var output bytes.Buffer
		auth := &terminalAuthenticator{
			reader: strings.NewReader(input),
			writer: &output,
		}

		pwd, err := auth.Password(ctx)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if pwd != "interactive_pass" {
			t.Errorf("expected 'interactive_pass', got %q", pwd)
		}
		if !strings.Contains(output.String(), "Enter Password:") {
			t.Error("expected output to contain prompt")
		}
	})

	t.Run("Code Interactive", func(t *testing.T) {
		input := "12345\n"
		var output bytes.Buffer
		auth := &terminalAuthenticator{
			reader: strings.NewReader(input),
			writer: &output,
		}

		code, err := auth.Code(ctx, nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if code != "12345" {
			t.Errorf("expected code '12345', got %q", code)
		}
		if !strings.Contains(output.String(), "Enter Code:") {
			t.Error("expected output to contain prompt")
		}
	})

	t.Run("SignUp Not Supported", func(t *testing.T) {
		auth := &terminalAuthenticator{}
		_, err := auth.SignUp(ctx)
		if err == nil {
			t.Error("expected error for SignUp, got nil")
		}
	})

	t.Run("Terms Are Accepted", func(t *testing.T) {
		auth := &terminalAuthenticator{}
		if err := auth.AcceptTermsOfService(ctx, tg.HelpTermsOfService{}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	// A closed stdin reaches Fscanln as EOF, which is what a login in a
	// container with no terminal attached does.
	t.Run("Password Without Input", func(t *testing.T) {
		var output bytes.Buffer
		auth := &terminalAuthenticator{
			reader: strings.NewReader(""),
			writer: &output,
		}
		if _, err := auth.Password(ctx); err == nil {
			t.Error("expected an error when nothing can be read")
		}
	})

	t.Run("Code Without Input", func(t *testing.T) {
		var output bytes.Buffer
		auth := &terminalAuthenticator{
			reader: strings.NewReader(""),
			writer: &output,
		}
		if _, err := auth.Code(ctx, nil); err == nil {
			t.Error("expected an error when nothing can be read")
		}
	})

	t.Run("Password With A Failing Writer", func(t *testing.T) {
		auth := &terminalAuthenticator{
			reader: strings.NewReader("pass\n"),
			writer: &flakyWriter{},
		}
		if _, err := auth.Password(ctx); err == nil {
			t.Error("expected the write error to surface")
		}
	})

	t.Run("Code With A Failing Writer", func(t *testing.T) {
		auth := &terminalAuthenticator{
			reader: strings.NewReader("123\n"),
			writer: &flakyWriter{},
		}
		if _, err := auth.Code(ctx, nil); err == nil {
			t.Error("expected the write error to surface")
		}
	})

	// Each prompt is two writes, an explanation and the prompt itself, so a
	// terminal that dies between them has to surface too.
	t.Run("Password With A Writer That Dies On The Prompt", func(t *testing.T) {
		auth := &terminalAuthenticator{
			reader: strings.NewReader("pass\n"),
			writer: &flakyWriter{ok: 1},
		}
		if _, err := auth.Password(ctx); err == nil {
			t.Error("expected the second write error to surface")
		}
	})

	t.Run("Code With A Writer That Dies On The Prompt", func(t *testing.T) {
		auth := &terminalAuthenticator{
			reader: strings.NewReader("123\n"),
			writer: &flakyWriter{ok: 1},
		}
		if _, err := auth.Code(ctx, nil); err == nil {
			t.Error("expected the second write error to surface")
		}
	})
}

// Accept ok writes, then fail every one after them.
type flakyWriter struct{ ok int }

func (w *flakyWriter) Write(p []byte) (int, error) {
	if w.ok > 0 {
		w.ok--
		return len(p), nil
	}
	return 0, errors.New("prompt could not be written")
}
