// Copyright © 2026 Bank-Vaults Maintainers
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

package common

import (
	"log/slog"
	"strconv"
	"strings"

	"github.com/spf13/viper"
)

// SplitAndTrim splits a comma-separated value, dropping empty entries.
func SplitAndTrim(csv string) []string {
	if strings.TrimSpace(csv) == "" {
		return nil
	}

	parts := strings.Split(csv, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if trimmed := strings.TrimSpace(p); trimmed != "" {
			out = append(out, trimmed)
		}
	}

	return out
}

// ResolveObjectSkipVerify honors an object-supplied skip-verify annotation only
// when the operator opted in (allowEnvVar); otherwise it returns the operator default.
func ResolveObjectSkipVerify(annotationValue, allowEnvVar, defaultEnvVar string) bool {
	requested, _ := strconv.ParseBool(annotationValue)
	if viper.GetBool(allowEnvVar) {
		return requested
	}

	if requested {
		slog.Warn("ignoring object skip-verify annotation; operator opt-in required",
			slog.String("allow_env", allowEnvVar))
	}

	return viper.GetBool(defaultEnvVar)
}
