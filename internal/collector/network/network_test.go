package network

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/arpingblue/AIStat/internal/collector"
	"github.com/arpingblue/AIStat/internal/fsx"
	"github.com/arpingblue/AIStat/internal/model"
)

func TestCollectFixture(t *testing.T) {
	root := filepath.Join("..", "..", "..", "testdata", "fixtures", "linux", "ubuntu2404-single-numa")
	result := Collector{}.Collect(context.Background(), collector.Env{FileSystem: fsx.Rooted{Root: root}, Platform: "windows", Fixture: true})
	if result.State != model.StateAvailable || len(result.Facts) != 2 {
		t.Fatalf("unexpected result: %#v", result)
	}
	network, ok := collector.DecodeFact[model.NetworkState](collector.Env{Facts: map[string]model.Fact{"network": result.Facts[0]}}, "network")
	if !ok || len(network.NICs) != 1 || network.NICs[0].Name != "eth0" {
		t.Fatalf("unexpected network fact: %#v", network)
	}
}
