package process

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/arpingblue/AIStat/internal/fsx"
)

func TestAllowedArgsAndEnvBoundary(t *testing.T) {
	args := []string{"python", "-m", "vllm.entrypoints", "--tensor-parallel-size", "4", "--device-ids=0,1", "--model", "/home/customer/private-model", "--api-key", "secret", "--hf-token", "hf_secret", "--aws-secret-access-key", "aws_secret"}
	got := parseAllowedArgs(args)
	if got["--tensor-parallel-size"] != "4" {
		t.Fatalf("missing safe arg: %v", got)
	}
	if _, ok := got["--api-key"]; ok {
		t.Fatal("secret-bearing arg escaped allowlist")
	}
	if got["--model"] != "/models/[REDACTED]" {
		t.Fatalf("model path not sanitized: %v", got)
	}
	joined := strings.Join(redactArgs(args), " ")
	for _, secret := range []string{"secret", "hf_secret", "aws_secret", "--api-key", "--hf-token", "customer", "private-model"} {
		if strings.Contains(joined, secret) {
			t.Fatalf("secret %q escaped redaction: %q", secret, joined)
		}
	}
}
func TestReadAllowedEnvExcludesSecrets(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "proc", "42")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	raw := []byte("CUDA_VISIBLE_DEVICES=0,1\x00HF_TOKEN=hf_secret\x00AWS_SECRET_ACCESS_KEY=aws_secret\x00API_KEY=api_secret\x00")
	if err := os.WriteFile(filepath.Join(dir, "environ"), raw, 0o600); err != nil {
		t.Fatal(err)
	}
	got, err := ReadAllowedEnv(fsx.Rooted{Root: root}, 42, allowedEnv)
	if err != nil || len(got) != 1 || got["CUDA_VISIBLE_DEVICES"] != "0,1" {
		t.Fatalf("unexpected allowed env: %v err=%v", got, err)
	}
	joined := fmt.Sprint(got)
	for _, secret := range []string{"hf_secret", "aws_secret", "api_secret"} {
		if strings.Contains(joined, secret) {
			t.Fatalf("secret %q escaped allowlist", secret)
		}
	}
}
func TestContainerID(t *testing.T) {
	got := containerID("0::/system.slice/docker-0123456789abcdef.scope\n")
	if got != "0123456789abcdef" || shortID(got) != "0123456789ab" {
		t.Fatalf("unexpected container ID %q", got)
	}
}

func TestDetectRuntimeKindBeforeRedaction(t *testing.T) {
	tests := []struct {
		args []string
		want string
	}{
		{[]string{"vllm", "serve", "model"}, "vllm"},
		{[]string{"python", "-m", "vllm.entrypoints.openai.api_server"}, "vllm"},
		{[]string{"python", "/opt/sglang/launch_server.py"}, "sglang"},
		{[]string{"torchrun", "serve.py"}, "pytorch"},
		{[]string{"python", "-m", "torch.distributed.run"}, "pytorch"},
		{[]string{"python", "my-vllm-notes.py"}, ""},
	}
	for _, test := range tests {
		if got := detectRuntimeKind(test.args); got != test.want {
			t.Fatalf("args=%v got=%q want=%q", test.args, got, test.want)
		}
	}
}

func FuzzParseAllowedArgs(f *testing.F) {
	f.Add("--tensor-parallel-size", "4")
	f.Add("--api-key", "secret")
	f.Add("--model", "/home/customer/model")
	f.Fuzz(func(t *testing.T, key, value string) {
		got := parseAllowedArgs([]string{"python", key, value})
		for parsedKey, parsedValue := range got {
			if !allowedFlags[parsedKey] {
				t.Fatalf("non-allowlisted key retained: %q", parsedKey)
			}
			if parsedKey == "--model" || parsedKey == "--model-path" {
				if parsedValue == value && value != "" {
					t.Fatalf("raw model path retained: %q", parsedValue)
				}
			}
		}
	})
}
