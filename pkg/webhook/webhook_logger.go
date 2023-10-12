// Copyright © 2023 Bank-Vaults Maintainers
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package webhook

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/slok/kubewebhook/v2/pkg/log"
)

var _ log.Logger = &whLogger{}

type whLogger struct {
	*slog.Logger
}

// NewWhLogger returns a new log.Logger for a slog implementation.
func NewWhLogger(l *slog.Logger) log.Logger {
	return whLogger{l}
}

func (l whLogger) Infof(format string, args ...interface{}) {
	l.Info(fmt.Sprintf(format, args...))
}

func (l whLogger) Warningf(format string, args ...interface{}) {
	l.Warn(fmt.Sprintf(format, args...))
}

func (l whLogger) Errorf(format string, args ...interface{}) {
	l.Error(fmt.Sprintf(format, args...))
}

func (l whLogger) Debugf(format string, args ...interface{}) {
	l.Debug(fmt.Sprintf(format, args...))
}

func (l whLogger) WithValues(kv log.Kv) log.Logger {
	attributes := make([]any, 0, len(kv))
	for k, v := range kv {
		attributes = append(attributes, slog.Any(k, v))
	}
	return NewWhLogger(l.With(attributes...))
}

func (l whLogger) WithCtxValues(ctx context.Context) log.Logger {
	ctxValues := log.ValuesFromCtx(ctx)
	attributes := make([]any, 0, len(ctxValues))
	for k, v := range ctxValues {
		attributes = append(attributes, slog.Any(k, v))
	}
	return NewWhLogger(l.With(attributes...))
}

func (l whLogger) SetValuesOnCtx(parent context.Context, values log.Kv) context.Context {
	return log.CtxWithValues(parent, values)
}
