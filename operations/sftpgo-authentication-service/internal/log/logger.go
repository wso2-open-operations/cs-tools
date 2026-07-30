// Copyright (c) 2026 WSO2 LLC. (https://www.wso2.com).
//
// WSO2 LLC. licenses this file to you under the Apache License,
// Version 2.0 (the "License"); you may not use this file except
// in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing,
// software distributed under the License is distributed on an
// "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
// KIND, either express or implied.  See the License for the
// specific language governing permissions and limitations
// under the License.

package log

import (
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"strings"
)

// LogLevel represents the severity of a log message.
type LogLevel int

const (
	LogLevelTrace LogLevel = iota
	LogLevelDebug
	LogLevelInfo
	LogLevelWarn
	LogLevelError
)

type AppLogger struct {
	logger *log.Logger
	level  LogLevel
}

func NewAppLogger(levelStr string) *AppLogger {
	logger := log.New(os.Stdout, "", log.Ldate|log.Ltime|log.Lshortfile)
	level := parseLogLevel(levelStr)
	return &AppLogger{
		logger: logger,
		level:  level,
	}
}

// parseLogLevel parses a string into a LogLevel.
func parseLogLevel(levelStr string) LogLevel {
	switch strings.ToUpper(levelStr) {
	case "TRACE":
		return LogLevelTrace
	case "DEBUG":
		return LogLevelDebug
	case "INFO":
		return LogLevelInfo
	case "WARN":
		return LogLevelWarn
	case "ERROR":
		return LogLevelError
	default:
		return LogLevelInfo // Default to INFO
	}
}

// SetOutput sets the output destination for the logger.
func (l *AppLogger) SetOutput(w io.Writer) {
	l.logger.SetOutput(w)
}

// Trace logs a message at the TRACE level.
func (l *AppLogger) Trace(format string, v ...interface{}) {
	if l.level <= LogLevelTrace {
		l.logger.Printf("[TRACE] "+format, v...)
	}
}

// Debug logs a message at the DEBUG level.
func (l *AppLogger) Debug(format string, v ...interface{}) {
	if l.level <= LogLevelDebug {
		l.logger.Printf("[DEBUG] "+format, v...)
	}
}

// Info logs a message at the INFO level.
func (l *AppLogger) Info(format string, v ...interface{}) {
	if l.level <= LogLevelInfo {
		l.logger.Printf("[INFO] "+format, v...)
	}
}

// Warn logs a message at the WARN level.
func (l *AppLogger) Warn(format string, v ...interface{}) {
	if l.level <= LogLevelWarn {
		l.logger.Printf("[WARN] "+format, v...)
	}
}

// Error logs a message at the ERROR level.
func (l *AppLogger) Error(format string, v ...interface{}) {
	if l.level <= LogLevelError {
		l.logger.Printf("[ERROR] "+format, v...)
	}
}

// Errorf logs a formatted message at the ERROR level and returns an error with the same message.
func (l *AppLogger) Errorf(format string, v ...interface{}) error {
	msg := fmt.Sprintf(format, v...)
	l.Error("%s", msg)
	return errors.New(msg)
}

// Fatal logs a message at the FATAL level and terminates the application.
func (l *AppLogger) Fatal(format string, v ...interface{}) {
	l.logger.Fatalf("[FATAL] "+format, v...)
}

// IsDebugEnabled returns true if the DEBUG log level is enabled.
func (l *AppLogger) IsDebugEnabled() bool {
	return l.level <= LogLevelDebug
}

// IsTraceEnabled returns true if the TRACE log level is enabled.
func (l *AppLogger) IsTraceEnabled() bool {
	return l.level <= LogLevelTrace
}
