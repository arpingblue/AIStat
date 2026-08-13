package collector

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/arpingblue/AIStat/internal/model"
)

type Registry struct{ collectors map[ID]Collector }

func NewRegistry(items ...Collector) (*Registry, error) {
	r := &Registry{collectors: make(map[ID]Collector, len(items))}
	for _, item := range items {
		if item == nil {
			return nil, fmt.Errorf("nil collector")
		}
		if _, exists := r.collectors[item.ID()]; exists {
			return nil, fmt.Errorf("duplicate collector %q", item.ID())
		}
		r.collectors[item.ID()] = item
	}
	return r, nil
}

func (r *Registry) Order() ([]Collector, error) {
	providers := map[Capability]ID{}
	for id, item := range r.collectors {
		for _, capability := range item.Provides() {
			if prior, exists := providers[capability]; exists {
				return nil, fmt.Errorf("capability %q provided by %q and %q", capability, prior, id)
			}
			providers[capability] = id
		}
	}
	remaining := map[ID]Collector{}
	for id, item := range r.collectors {
		remaining[id] = item
	}
	available := map[Capability]bool{}
	ordered := make([]Collector, 0, len(remaining))
	for len(remaining) > 0 {
		ids := make([]string, 0, len(remaining))
		for id := range remaining {
			ids = append(ids, string(id))
		}
		sort.Strings(ids)
		progress := false
		for _, rawID := range ids {
			id := ID(rawID)
			item := remaining[id]
			ready := true
			for _, requirement := range item.Requires() {
				if _, known := providers[requirement]; !known {
					return nil, fmt.Errorf("collector %q requires unknown capability %q", id, requirement)
				}
				if !available[requirement] {
					ready = false
					break
				}
			}
			if !ready {
				continue
			}
			ordered = append(ordered, item)
			delete(remaining, id)
			progress = true
			for _, capability := range item.Provides() {
				available[capability] = true
			}
		}
		if !progress {
			return nil, fmt.Errorf("collector dependency cycle")
		}
	}
	return ordered, nil
}

func (r *Registry) Run(ctx context.Context, env Env) ([]Result, []model.CollectorStatus, error) {
	if _, err := r.Order(); err != nil {
		return nil, nil, err
	}
	if env.Facts == nil {
		env.Facts = map[string]model.Fact{}
	}
	results := []Result{}
	statuses := []model.CollectorStatus{}
	pending := map[ID]Collector{}
	for id, item := range r.collectors {
		pending[id] = item
	}
	available := map[Capability]bool{}
	for len(pending) > 0 {
		ready := []Collector{}
		for _, item := range pending {
			ok := true
			for _, required := range item.Requires() {
				if !available[required] {
					ok = false
					break
				}
			}
			if ok {
				ready = append(ready, item)
			}
		}
		sort.Slice(ready, func(i, j int) bool { return ready[i].ID() < ready[j].ID() })
		if len(ready) == 0 {
			return nil, nil, fmt.Errorf("collector dependency cycle")
		}
		for start := 0; start < len(ready); start += 8 {
			end := min(start+8, len(ready))
			batch := ready[start:end]
			type outcome struct {
				result Result
				status model.CollectorStatus
			}
			outcomes := make([]outcome, len(batch))
			var wait sync.WaitGroup
			for i, item := range batch {
				wait.Add(1)
				go func(i int, item Collector) {
					defer wait.Done()
					started := time.Now()
					result := safeCollect(ctx, item, env)
					if result.Collector == "" {
						result.Collector = item.ID()
					}
					status := model.CollectorStatus{ID: string(item.ID()), State: result.State, DurationMS: time.Since(started).Milliseconds()}
					if len(result.Diagnostics) > 0 {
						status.Error = result.Diagnostics[0].Message
					}
					outcomes[i] = outcome{result, status}
				}(i, item)
			}
			wait.Wait()
			for _, outcome := range outcomes {
				results = append(results, outcome.result)
				statuses = append(statuses, outcome.status)
				for _, fact := range outcome.result.Facts {
					env.Facts[fact.Key] = fact
				}
				item := pending[outcome.result.Collector]
				for _, capability := range item.Provides() {
					available[capability] = true
				}
				delete(pending, outcome.result.Collector)
			}
		}
	}
	return results, statuses, nil
}

func safeCollect(ctx context.Context, item Collector, env Env) (result Result) {
	defer func() {
		if recovered := recover(); recovered != nil {
			result = Result{Collector: item.ID(), State: model.StateUnknown, Diagnostics: []model.Diagnostic{{Level: "error", Code: "collector_panic", Message: fmt.Sprint(recovered)}}}
		}
	}()
	return item.Collect(ctx, env)
}
