package httpc

import (
	"crypto/tls"
	"testing"
)

// FuzzParseTLSVersion ensures parseTLSVersion handles arbitrary strings safely
// and only returns known TLS versions or 0.
func FuzzParseTLSVersion(f *testing.F) {
	f.Add("")
	f.Add("1.2")
	f.Add("tls1.3")
	f.Add("TLS13")
	f.Add("weird-input!!")

	f.Fuzz(func(t *testing.T, s string) {
		v := parseTLSVersion(s)
		if v != 0 && v != tls.VersionTLS10 && v != tls.VersionTLS11 && v != tls.VersionTLS12 && v != tls.VersionTLS13 {
			t.Fatalf("unexpected tls version: %v", v)
		}
	})
}
