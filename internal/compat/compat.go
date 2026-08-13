package compat

import (
	"embed"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"
)

//go:embed data/*.json
var datasets embed.FS

var DatasetVersion string

var Sources []string

type Decision string

const (
	Compatible            Decision = "compatible"
	CompatibleWithPackage Decision = "compatible_with_package"
	Incompatible          Decision = "incompatible"
	Unknown               Decision = "unknown"
)

var minimumDriver map[int]string

func init() {
	if err := loadDatasets(); err != nil {
		panic("load embedded compatibility data: " + err.Error())
	}
}

func loadDatasets() error {
	var cudaData struct {
		SchemaVersion  string            `json:"schema_version"`
		DatasetVersion string            `json:"dataset_version"`
		Source         string            `json:"source"`
		Minimum        map[string]string `json:"minimum_driver_by_cuda_major"`
	}
	raw, err := datasets.ReadFile("data/cuda.json")
	if err != nil {
		return err
	}
	if err := json.Unmarshal(raw, &cudaData); err != nil {
		return err
	}
	if cudaData.SchemaVersion != "0.1" || cudaData.DatasetVersion == "" || cudaData.Source == "" {
		return fmt.Errorf("invalid CUDA dataset metadata")
	}
	minimumDriver = make(map[int]string, len(cudaData.Minimum))
	for rawMajor, minimum := range cudaData.Minimum {
		parsed, parseErr := strconv.Atoi(rawMajor)
		if parseErr != nil || minimum == "" {
			return fmt.Errorf("invalid CUDA compatibility entry %q", rawMajor)
		}
		minimumDriver[parsed] = minimum
	}

	var lifecycleData struct {
		SchemaVersion  string `json:"schema_version"`
		DatasetVersion string `json:"dataset_version"`
		Source         string `json:"source"`
		Branches       []struct {
			Branch int    `json:"branch"`
			EOL    string `json:"eol"`
		} `json:"branches"`
	}
	raw, err = datasets.ReadFile("data/nvidia-driver-lifecycle.json")
	if err != nil {
		return err
	}
	if err := json.Unmarshal(raw, &lifecycleData); err != nil {
		return err
	}
	if lifecycleData.SchemaVersion != "0.1" || lifecycleData.DatasetVersion != cudaData.DatasetVersion || lifecycleData.Source == "" {
		return fmt.Errorf("compatibility dataset versions do not match")
	}
	lifecycles = make([]Lifecycle, 0, len(lifecycleData.Branches))
	for _, row := range lifecycleData.Branches {
		date, parseErr := time.Parse("2006-01-02", row.EOL)
		if parseErr != nil || row.Branch <= 0 {
			return fmt.Errorf("invalid lifecycle entry for branch %d", row.Branch)
		}
		lifecycles = append(lifecycles, Lifecycle{Branch: row.Branch, EOL: date})
	}
	DatasetVersion = cudaData.DatasetVersion
	Sources = []string{cudaData.Source, lifecycleData.Source}
	return nil
}

func CUDA(driver, cuda string) (Decision, string) {
	return CUDAWithCompat(driver, cuda, false)
}

func CUDAWithCompat(driver, cuda string, compatibilityPackage bool) (Decision, string) {
	major, ok := major(cuda)
	if !ok {
		return Unknown, "CUDA version is not parseable"
	}
	minimum, supported := minimumDriver[major]
	if !supported {
		return Unknown, "CUDA major is outside the embedded dataset"
	}
	comparison, ok := compare(driver, minimum)
	if !ok {
		return Unknown, "driver version is not parseable"
	}
	if comparison < 0 {
		if compatibilityPackage {
			return CompatibleWithPackage, fmt.Sprintf("driver %s is below native minimum %s for CUDA %d.x, but a CUDA forward-compatibility package was detected", driver, minimum, major)
		}
		return Incompatible, fmt.Sprintf("driver %s is below minimum compatibility driver %s for CUDA %d.x", driver, minimum, major)
	}
	return Compatible, fmt.Sprintf("driver %s meets minimum compatibility driver %s for CUDA %d.x", driver, minimum, major)
}

type Lifecycle struct {
	Branch int
	EOL    time.Time
}

var lifecycles []Lifecycle

func DriverEOL(driver string, now time.Time) (bool, time.Time, bool) {
	branch, ok := major(driver)
	if !ok {
		return false, time.Time{}, false
	}
	for _, item := range lifecycles {
		if item.Branch == branch {
			return !now.Before(item.EOL), item.EOL, true
		}
	}
	return false, time.Time{}, false
}

func major(value string) (int, bool) {
	first := strings.SplitN(strings.TrimSpace(strings.TrimPrefix(value, "v")), ".", 2)[0]
	parsed, err := strconv.Atoi(first)
	return parsed, err == nil
}
func compare(a, b string) (int, bool) {
	pa, oka := parts(a)
	pb, okb := parts(b)
	if !oka || !okb {
		return 0, false
	}
	for i := 0; i < 3; i++ {
		if pa[i] < pb[i] {
			return -1, true
		}
		if pa[i] > pb[i] {
			return 1, true
		}
	}
	return 0, true
}
func parts(value string) ([3]int, bool) {
	var result [3]int
	fields := strings.Split(strings.TrimSpace(value), ".")
	if len(fields) < 2 {
		return result, false
	}
	for i := 0; i < len(fields) && i < 3; i++ {
		digits := fields[i]
		for j, char := range digits {
			if char < '0' || char > '9' {
				digits = digits[:j]
				break
			}
		}
		if digits == "" {
			return result, false
		}
		parsed, err := strconv.Atoi(digits)
		if err != nil {
			return result, false
		}
		result[i] = parsed
	}
	return result, true
}
