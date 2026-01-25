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
	"os"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// Create new zap logger configured for console output.
// Direct Info level and above to stdout, and Error level and above to stderr.
func New() (*zap.Logger, error) {
	// Configure encoder
	encoderConfig := zap.NewProductionEncoderConfig()

	// Format time
	encoderConfig.EncodeTime = func(t time.Time, enc zapcore.PrimitiveArrayEncoder) {
		enc.AppendString("[" + t.Format(time.RFC3339Nano) + "]")
	}

	// Format level: [INFO]
	encoderConfig.EncodeLevel = func(l zapcore.Level, enc zapcore.PrimitiveArrayEncoder) {
		enc.AppendString("[" + l.CapitalString() + "]")
	}

	// Remove caller information
	encoderConfig.EncodeCaller = nil

	// Use spaces instead of tabs for separation
	encoderConfig.ConsoleSeparator = " "

	// Use ConsoleEncoder instead of JSON for better readability
	encoder := zapcore.NewConsoleEncoder(encoderConfig)

	// Direct high priority logs (Error, Panic, Fatal) to stderr
	highPriority := zap.LevelEnablerFunc(func(lvl zapcore.Level) bool {
		return lvl >= zapcore.ErrorLevel
	})

	// Direct low priority logs (Info, Warn) to stdout
	lowPriority := zap.LevelEnablerFunc(func(lvl zapcore.Level) bool {
		return lvl >= zapcore.InfoLevel && lvl < zapcore.ErrorLevel
	})

	// Lock streams to prevent race conditions on writes
	consoleOut := zapcore.Lock(os.Stdout)
	consoleErr := zapcore.Lock(os.Stderr)

	// Combine cores to split output based on level
	core := zapcore.NewTee(
		zapcore.NewCore(encoder, consoleErr, highPriority),
		zapcore.NewCore(encoder, consoleOut, lowPriority),
	)

	// Build logger without AddCaller option
	return zap.New(core), nil
}
