package memory

import "testing"

func TestParse(t *testing.T) {
	memory, err := Parse("MemTotal: 1024 kB\nMemAvailable: 256 kB\nSwapTotal: 128 kB\nSwapFree: 64 kB\nHugePages_Total: 4\nHugePages_Free: 2\nHugepagesize: 2048 kB\n")
	if err != nil {
		t.Fatal(err)
	}
	if memory.TotalBytes != 1024*1024 || memory.FreeBytes != 256*1024 || memory.SwapTotalBytes != 128*1024 || memory.HugePagesTotal != 4 || memory.HugePagesFree != 2 || memory.HugePageSizeBytes != 2048*1024 {
		t.Fatalf("unexpected memory: %#v", memory)
	}
}
func TestTHPAndMemlockParsers(t *testing.T) {
	if got := selected("always madvise [never]"); got != "never" {
		t.Fatalf("unexpected THP mode %q", got)
	}
	soft, hard, softUnlimited, hardUnlimited := parseMemlock("Max locked memory         65536                unlimited            bytes\n")
	if soft == nil || *soft != 65536 || hard != nil || softUnlimited || !hardUnlimited {
		t.Fatalf("unexpected memlock: soft=%v hard=%v softUnlimited=%v hardUnlimited=%v", soft, hard, softUnlimited, hardUnlimited)
	}
}
func TestParseRequiresTotal(t *testing.T) {
	if _, err := Parse("MemFree: 1 kB\n"); err == nil {
		t.Fatal("expected error")
	}
}

func FuzzParseMeminfo(f *testing.F) {
	f.Add("MemTotal: 1024 kB\nMemAvailable: 512 kB\n")
	f.Add("")
	f.Fuzz(func(t *testing.T, raw string) {
		memory, err := Parse(raw)
		if err != nil {
			return
		}
		if memory.State != "available" || memory.TotalBytes == 0 {
			t.Fatalf("invalid successful parse: %#v", memory)
		}
	})
}
