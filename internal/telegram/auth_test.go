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
	"strings"
	"testing"
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
}
