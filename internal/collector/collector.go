package collector

import (
	"context"
	"encoding/json"
	"errors"
	"io/fs"

	"github.com/arpingblue/AIStat/internal/clock"
	"github.com/arpingblue/AIStat/internal/execx"
	"github.com/arpingblue/AIStat/internal/fsx"
	"github.com/arpingblue/AIStat/internal/model"
)

func FileErrorState(err error) model.FactState {
	if errors.Is(err, fs.ErrPermission) {
		return model.StatePermissionDenied
	}
	if errors.Is(err, fs.ErrNotExist) {
		return model.StateNotDetected
	}
	return model.StateUnknown
}

type ID string
type Capability string

type Collector interface {
	ID() ID
	Provides() []Capability
	Requires() []Capability
	Collect(context.Context, Env) Result
}

type Env struct {
	Runner     execx.Runner
	FileSystem fsx.FileSystem
	Clock      clock.Clock
	Platform   string
	Fixture    bool
	Facts      map[string]model.Fact
}

type Result struct {
	Collector   ID
	State       model.FactState
	Facts       []model.Fact
	Diagnostics []model.Diagnostic
}

func DecodeFact[T any](env Env, key string) (T, bool) {
	var value T
	fact, ok := env.Facts[key]
	if !ok || fact.State != model.StateAvailable || len(fact.Value) == 0 {
		return value, false
	}
	if err := json.Unmarshal(fact.Value, &value); err != nil {
		return value, false
	}
	return value, true
}

func Unsupported(id ID, key string) Result {
	return Result{Collector: id, State: model.StateUnsupported, Facts: []model.Fact{{Key: key, State: model.StateUnsupported, Confidence: model.ConfidenceHigh}}}
}
