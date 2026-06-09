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
	"testing"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
)

func TestSplitAndTrim(t *testing.T) {
	assert.Nil(t, SplitAndTrim(""))
	assert.Nil(t, SplitAndTrim("   "))
	assert.Equal(t, []string{"a"}, SplitAndTrim("a"))
	assert.Equal(t, []string{"a", "b"}, SplitAndTrim(" a , b "))
	assert.Equal(t, []string{"a", "b"}, SplitAndTrim("a,,b,"))
}

func TestResolveObjectSkipVerify(t *testing.T) {
	const allowEnv = "test_allow_object_skip_verify"
	const defaultEnv = "test_skip_verify"

	tests := []struct {
		name            string
		annotationValue string
		allow           bool
		operatorDefault bool
		want            bool
	}{
		{name: "annotation true ignored by default falls back to operator default false", annotationValue: "true", allow: false, operatorDefault: false, want: false},
		{name: "annotation true ignored by default falls back to operator default true", annotationValue: "true", allow: false, operatorDefault: true, want: true},
		{name: "annotation true honored when opted in", annotationValue: "true", allow: true, operatorDefault: false, want: true},
		{name: "annotation false honored when opted in", annotationValue: "false", allow: true, operatorDefault: true, want: false},
		{name: "annotation false ignored by default falls back to operator default", annotationValue: "false", allow: false, operatorDefault: true, want: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			viper.Set(allowEnv, tt.allow)
			viper.Set(defaultEnv, tt.operatorDefault)
			t.Cleanup(viper.Reset)

			assert.Equal(t, tt.want, ResolveObjectSkipVerify(tt.annotationValue, allowEnv, defaultEnv))
		})
	}
}
