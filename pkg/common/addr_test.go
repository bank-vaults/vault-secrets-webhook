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

	"github.com/stretchr/testify/assert"
)

func TestValidateObjectAddr(t *testing.T) {
	tests := []struct {
		name    string
		addr    string
		policy  AddrPolicy
		wantErr bool
	}{
		{
			name:    "empty address is rejected",
			addr:    "",
			policy:  AddrPolicy{Allowlist: []string{"https://vault.prod.svc:8200"}},
			wantErr: true,
		},
		{
			name:    "allowlisted public https address is accepted",
			addr:    "https://vault.prod.svc:8200",
			policy:  AddrPolicy{Allowlist: []string{"https://vault.prod.svc:8200"}},
			wantErr: false,
		},
		{
			name:    "address not in allowlist is rejected (default-deny)",
			addr:    "https://evil.attacker.com",
			policy:  AddrPolicy{Allowlist: []string{"https://vault.prod.svc:8200"}},
			wantErr: true,
		},
		{
			name:    "empty allowlist denies every object override",
			addr:    "https://vault.prod.svc:8200",
			policy:  AddrPolicy{Allowlist: nil},
			wantErr: true,
		},
		{
			name:    "non-http scheme is rejected",
			addr:    "file:///etc/passwd",
			policy:  AddrPolicy{Allowlist: []string{"file:///etc/passwd"}},
			wantErr: true,
		},
		{
			name:    "address without scheme is rejected",
			addr:    "169.254.169.254",
			policy:  AddrPolicy{Allowlist: []string{"169.254.169.254"}},
			wantErr: true,
		},
		{
			name:    "IMDS link-local is rejected even when allowlisted",
			addr:    "http://169.254.169.254/latest/meta-data/",
			policy:  AddrPolicy{Allowlist: []string{"http://169.254.169.254/latest/meta-data/"}},
			wantErr: true,
		},
		{
			name:    "loopback IP literal is rejected by default",
			addr:    "http://127.0.0.1:8200",
			policy:  AddrPolicy{Allowlist: []string{"http://127.0.0.1:8200"}},
			wantErr: true,
		},
		{
			name:    "private RFC1918 IP literal is rejected by default",
			addr:    "https://10.0.0.5:8200",
			policy:  AddrPolicy{Allowlist: []string{"https://10.0.0.5:8200"}},
			wantErr: true,
		},
		{
			name:    "private IP literal is accepted when operator allows private",
			addr:    "https://10.0.0.5:8200",
			policy:  AddrPolicy{Allowlist: []string{"https://10.0.0.5:8200"}, AllowPrivate: true},
			wantErr: false,
		},
		{
			name:    "metadata hostname is rejected by default",
			addr:    "http://metadata.google.internal/computeMetadata/v1/",
			policy:  AddrPolicy{Allowlist: []string{"http://metadata.google.internal/computeMetadata/v1/"}},
			wantErr: true,
		},
		{
			name:    "localhost hostname is rejected by default",
			addr:    "http://localhost:8200",
			policy:  AddrPolicy{Allowlist: []string{"http://localhost:8200"}},
			wantErr: true,
		},
		{
			name:    "allowlist match is normalized (trailing slash and case)",
			addr:    "https://Vault.Prod.SVC:8200/",
			policy:  AddrPolicy{Allowlist: []string{"https://vault.prod.svc:8200"}},
			wantErr: false,
		},
		{
			name:    "cluster hostname override is accepted when allowlisted",
			addr:    "https://vault-dr.vault-system.svc:8200",
			policy:  AddrPolicy{Allowlist: []string{"https://vault.vault-system.svc:8200", "https://vault-dr.vault-system.svc:8200"}},
			wantErr: false,
		},
		{
			name:    "userinfo credentials are rejected even when host is allowlisted",
			addr:    "https://attacker:pw@vault.prod.svc:8200",
			policy:  AddrPolicy{Allowlist: []string{"https://vault.prod.svc:8200"}},
			wantErr: true,
		},
		{
			name:    "query string is rejected",
			addr:    "https://vault.prod.svc:8200?x=1",
			policy:  AddrPolicy{Allowlist: []string{"https://vault.prod.svc:8200"}},
			wantErr: true,
		},
		{
			name:    "fragment is rejected",
			addr:    "https://vault.prod.svc:8200#frag",
			policy:  AddrPolicy{Allowlist: []string{"https://vault.prod.svc:8200"}},
			wantErr: true,
		},
		{
			name:    "metadata hostname is rejected even when private addresses are allowed",
			addr:    "http://metadata.google.internal/computeMetadata/v1/",
			policy:  AddrPolicy{Allowlist: []string{"http://metadata.google.internal/computeMetadata/v1/"}, AllowPrivate: true},
			wantErr: true,
		},
		{
			name:    "localhost is rejected even when private addresses are allowed",
			addr:    "http://localhost:8200",
			policy:  AddrPolicy{Allowlist: []string{"http://localhost:8200"}, AllowPrivate: true},
			wantErr: true,
		},
		{
			name:    "decimal IP literal for loopback is rejected",
			addr:    "http://2130706433:8200",
			policy:  AddrPolicy{Allowlist: []string{"http://2130706433:8200"}},
			wantErr: true,
		},
		{
			name:    "octal IP literal for loopback is rejected",
			addr:    "http://0177.0.0.1:8200",
			policy:  AddrPolicy{Allowlist: []string{"http://0177.0.0.1:8200"}},
			wantErr: true,
		},
		{
			name:    "hex IP literal is rejected",
			addr:    "http://0x7f000001:8200",
			policy:  AddrPolicy{Allowlist: []string{"http://0x7f000001:8200"}},
			wantErr: true,
		},
		{
			name:    "CGNAT shared-address-space IMDS is rejected",
			addr:    "http://100.100.100.200/latest/meta-data/",
			policy:  AddrPolicy{Allowlist: []string{"http://100.100.100.200/latest/meta-data/"}},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateObjectAddr(tt.addr, tt.policy)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
