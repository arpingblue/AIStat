package redact

import (
	"strings"
	"testing"
)

func TestText(t *testing.T) {
	got := Text("ip=10.1.2.3 mac=aa:bb:cc:dd:ee:ff token=secret")
	for _, secret := range []string{"10.1.2.3", "aa:bb:cc:dd:ee:ff", "token=secret"} {
		if strings.Contains(got, secret) {
			t.Fatalf("secret %q remains in %q", secret, got)
		}
	}
}
func TestEnv(t *testing.T) {
	got := Env(map[string]string{"API_KEY": "secret", "NCCL_SOCKET_IFNAME": "eth0"})
	if got["API_KEY"] != "[REDACTED]" || got["NCCL_SOCKET_IFNAME"] != "eth0" {
		t.Fatalf("unexpected redaction: %v", got)
	}
}
func TestModelPath(t *testing.T) {
	for _, raw := range []string{"/home/customer/private-model", "customer/private-model", "private-model"} {
		if got := ModelPath(raw); strings.Contains(got, "customer") || strings.Contains(got, "private-model") {
			t.Fatalf("model path was not redacted: %q -> %q", raw, got)
		}
	}
}
