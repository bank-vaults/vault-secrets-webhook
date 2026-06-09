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
	"net"
	"net/url"
	"strings"

	"emperror.dev/errors"
)

// AddrPolicy constrains an address taken from an untrusted object annotation.
type AddrPolicy struct {
	Allowlist    []string
	AllowPrivate bool
}

var metadataHosts = map[string]struct{}{
	"localhost":                {},
	"metadata.google.internal": {},
}

// RFC 6598 shared address space (100.64.0.0/10), used by some cloud IMDS.
var cgnatRange = &net.IPNet{IP: net.IPv4(100, 64, 0, 0), Mask: net.CIDRMask(10, 32)}

// ValidateObjectAddr vets an object-supplied address against the operator
// allowlist and an SSRF guard. Operator-configured addresses are trusted and
// must not be passed here.
func ValidateObjectAddr(addr string, policy AddrPolicy) error {
	addr = strings.TrimSpace(addr)
	if addr == "" {
		return errors.New("address is empty")
	}

	u, err := url.Parse(addr)
	if err != nil {
		return errors.Wrapf(err, "malformed address %q", addr)
	}

	switch strings.ToLower(u.Scheme) {
	case "http", "https":
	default:
		return errors.Errorf("address %q has unsupported scheme %q (only http and https are allowed)", addr, u.Scheme)
	}

	host := u.Hostname()
	if host == "" {
		return errors.Errorf("address %q has no host", addr)
	}

	if u.User != nil {
		return errors.Errorf("address %q must not contain user credentials", addr)
	}

	if u.RawQuery != "" || u.ForceQuery || u.Fragment != "" {
		return errors.Errorf("address %q must not contain a query string or fragment", addr)
	}

	if !addrInAllowlist(addr, policy.Allowlist) {
		return errors.Errorf("address %q is not in the configured allowlist", addr)
	}

	// Metadata hosts are blocked unconditionally — AllowPrivate must not reach them.
	if _, ok := metadataHosts[strings.ToLower(host)]; ok {
		return errors.Errorf("address %q targets a metadata host, which is never allowed", addr)
	}

	if !policy.AllowPrivate {
		if err := checkHostIPSafety(host); err != nil {
			return errors.Wrapf(err, "address %q", addr)
		}
	}

	return nil
}

func addrInAllowlist(addr string, allowlist []string) bool {
	target := normalizeAddr(addr)
	for _, entry := range allowlist {
		if strings.TrimSpace(entry) == "" {
			continue
		}
		if normalizeAddr(entry) == target {
			return true
		}
	}

	return false
}

func normalizeAddr(addr string) string {
	u, err := url.Parse(strings.TrimSpace(addr))
	if err != nil {
		return strings.ToLower(strings.TrimSpace(addr))
	}

	return strings.ToLower(u.Scheme) + "://" + strings.ToLower(u.Host) + strings.TrimSuffix(u.Path, "/")
}

// checkHostIPSafety rejects loopback/link-local/private/CGNAT targets. DNS is
// not resolved here (to avoid resolve-time rebinding); hostnames rely on the allowlist.
func checkHostIPSafety(host string) error {
	ip := net.ParseIP(host)
	if ip == nil {
		if looksLikeNumericIP(host) {
			return errors.Errorf("has an ambiguous numeric IP literal host %q", host)
		}

		return nil
	}

	if isUnsafeIP(ip) {
		return errors.Errorf("targets a loopback, link-local, private, or shared-address-space host %q", host)
	}

	return nil
}

func isUnsafeIP(ip net.IP) bool {
	return ip.IsLoopback() ||
		ip.IsLinkLocalUnicast() ||
		ip.IsLinkLocalMulticast() ||
		ip.IsPrivate() ||
		ip.IsUnspecified() ||
		cgnatRange.Contains(ip)
}

// looksLikeNumericIP catches non-standard IPv4 forms (octal/decimal/hex) that
// net.ParseIP rejects but an OS resolver may still treat as an address.
func looksLikeNumericIP(host string) bool {
	if strings.HasPrefix(strings.ToLower(host), "0x") {
		return true
	}

	hasDigit := false
	for _, r := range host {
		switch {
		case r >= '0' && r <= '9':
			hasDigit = true
		case r == '.':
		default:
			return false
		}
	}

	return hasDigit
}
