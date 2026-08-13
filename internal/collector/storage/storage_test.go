package storage

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
	if result.State != model.StateAvailable {
		t.Fatalf("unexpected result: %#v", result)
	}
	storage, ok := collector.DecodeFact[model.StorageState](collector.Env{Facts: map[string]model.Fact{"storage": result.Facts[0]}}, "storage")
	if !ok || len(storage.Mounts) != 2 || storage.Mounts[0].Target != "/" {
		t.Fatalf("unexpected storage fact: %#v", storage)
	}
}
