package collector

import (
	"context"
	"fmt"
	"io/fs"
	"sync/atomic"
	"testing"
	"time"

	"github.com/arpingblue/AIStat/internal/model"
)

func TestFileErrorState(t *testing.T) {
	if got := FileErrorState(fs.ErrPermission); got != model.StatePermissionDenied {
		t.Fatalf("permission classified as %s", got)
	}
	if got := FileErrorState(fs.ErrNotExist); got != model.StateNotDetected {
		t.Fatalf("not-exist classified as %s", got)
	}
}

type fakeCollector struct {
	id                 ID
	provides, requires []Capability
}

func (f fakeCollector) ID() ID                 { return f.id }
func (f fakeCollector) Provides() []Capability { return f.provides }
func (f fakeCollector) Requires() []Capability { return f.requires }
func (f fakeCollector) Collect(context.Context, Env) Result {
	return Result{Collector: f.id, State: model.StateAvailable}
}
func TestRegistryTopologicalOrder(t *testing.T) {
	registry, err := NewRegistry(fakeCollector{"b", []Capability{"b"}, []Capability{"a"}}, fakeCollector{"a", []Capability{"a"}, nil})
	if err != nil {
		t.Fatal(err)
	}
	ordered, err := registry.Order()
	if err != nil {
		t.Fatal(err)
	}
	if ordered[0].ID() != "a" || ordered[1].ID() != "b" {
		t.Fatalf("bad order: %s %s", ordered[0].ID(), ordered[1].ID())
	}
}
func TestRegistryRejectsCycle(t *testing.T) {
	registry, _ := NewRegistry(fakeCollector{"a", []Capability{"a"}, []Capability{"b"}}, fakeCollector{"b", []Capability{"b"}, []Capability{"a"}})
	if _, err := registry.Order(); err == nil {
		t.Fatal("expected cycle")
	}
}

type concurrentCollector struct {
	id              ID
	active, maximum *int32
}

func (c concurrentCollector) ID() ID                 { return c.id }
func (c concurrentCollector) Provides() []Capability { return []Capability{Capability(c.id)} }
func (c concurrentCollector) Requires() []Capability { return nil }
func (c concurrentCollector) Collect(context.Context, Env) Result {
	active := atomic.AddInt32(c.active, 1)
	for {
		prior := atomic.LoadInt32(c.maximum)
		if active <= prior || atomic.CompareAndSwapInt32(c.maximum, prior, active) {
			break
		}
	}
	time.Sleep(20 * time.Millisecond)
	atomic.AddInt32(c.active, -1)
	return Result{Collector: c.id, State: model.StateAvailable}
}
func TestRegistryRunsAtMostEightCollectorsConcurrently(t *testing.T) {
	var active, maximum int32
	items := make([]Collector, 10)
	for i := range items {
		items[i] = concurrentCollector{id: ID(fmt.Sprintf("c%02d", i)), active: &active, maximum: &maximum}
	}
	registry, err := NewRegistry(items...)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := registry.Run(context.Background(), Env{}); err != nil {
		t.Fatal(err)
	}
	if maximum <= 1 || maximum > 8 {
		t.Fatalf("maximum concurrency=%d, want 2..8", maximum)
	}
}
