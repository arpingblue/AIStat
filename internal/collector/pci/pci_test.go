package pci

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/arpingblue/AIStat/internal/collector"
	"github.com/arpingblue/AIStat/internal/fsx"
	"github.com/arpingblue/AIStat/internal/model"
)

func TestParseACS(t *testing.T) {
	raw := "0000:00:01.0 PCI bridge: Fixture\n\tACSCtl: SrcValid+ TransBlk- ReqRedir+ CmpltRedir- UpstreamFwd-\n0000:3b:00.0 VGA controller: Fixture\n"
	got := ParseACS(raw)
	if redirect, ok := got["0000:00:01.0"]; !ok || !redirect {
		t.Fatalf("unexpected ACS: %v", got)
	}
}

func TestCollectFixture(t *testing.T) {
	root := filepath.Join("..", "..", "..", "testdata", "fixtures", "linux", "ubuntu2404-single-numa")
	result := Collector{}.Collect(context.Background(), collector.Env{FileSystem: fsx.Rooted{Root: root}, Platform: "windows", Fixture: true})
	if result.State != model.StateAvailable {
		t.Fatalf("unexpected result: %#v", result)
	}
	state, ok := collector.DecodeFact[model.PCIState](collector.Env{Facts: map[string]model.Fact{"pci": result.Facts[0]}}, "pci")
	if !ok || len(state.Devices) != 1 || state.Devices[0].NUMANode == nil || *state.Devices[0].NUMANode != 0 {
		t.Fatalf("unexpected PCI fact: %#v", state)
	}
}
