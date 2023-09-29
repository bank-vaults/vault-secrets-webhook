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
	"log/slog"

	"github.com/bank-vaults/vault-sdk/vault"
)

var _ vault.Logger = &clientLogger{}

type clientLogger struct {
	logger *slog.Logger
}

func (l clientLogger) Trace(msg string, args ...map[string]interface{}) {
	l.Debug(msg, args...)
}

func (l clientLogger) Debug(msg string, args ...map[string]interface{}) {
	l.logger.Debug(msg, l.argsToAttrs(args...)...)
}

func (l clientLogger) Info(msg string, args ...map[string]interface{}) {
	l.logger.Info(msg, l.argsToAttrs(args...)...)
}

func (l clientLogger) Warn(msg string, args ...map[string]interface{}) {
	l.logger.Warn(msg, l.argsToAttrs(args...)...)
}

func (l clientLogger) Error(msg string, args ...map[string]interface{}) {
	l.logger.Error(msg, l.argsToAttrs(args...)...)
}

func (clientLogger) argsToAttrs(args ...map[string]interface{}) []any {
	var attrs []any

	for _, arg := range args {
		for key, value := range arg {
			attrs = append(attrs, slog.Any(key, value))
		}
	}

	return attrs
}
