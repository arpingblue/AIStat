package system

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/arpingblue/AIStat/internal/collector"
	"github.com/arpingblue/AIStat/internal/fsx"
	"github.com/arpingblue/AIStat/internal/model"
)

func TestParseOSRelease(t *testing.T) {
	got := parseOSRelease("NAME=Ubuntu\nPRETTY_NAME=\"Ubuntu 24.04 LTS\"\n# comment\n")
	if got["PRETTY_NAME"] != "Ubuntu 24.04 LTS" {
		t.Fatalf("unexpected parse: %v", got)
	}
}

func TestCollectFixture(t *testing.T) {
	root := filepath.Join("..", "..", "..", "testdata", "fixtures", "linux", "ubuntu2404-single-numa")
	result := Collector{}.Collect(context.Background(), collector.Env{FileSystem: fsx.Rooted{Root: root}, Platform: "linux", Fixture: true})
	if result.State != model.StateAvailable || len(result.Facts) != 1 {
		t.Fatalf("unexpected result: %#v", result)
	}
}
