package python

import (
	"context"
	"io/fs"
	"path"
	"sort"
	"strings"
	"time"

	"github.com/arpingblue/AIStat/internal/fsx"
	"github.com/arpingblue/AIStat/internal/model"
)

const (
	installationScanBudget = 2 * time.Second
	maxEnvironments        = 128
	maxDirectoryEntries    = 4096
	maxMetadataBytes       = 256 << 10
)

type searchPrefix struct {
	Path        string
	Scope       string
	ContainerID string
	LogicalRoot string
	Children    bool
}

type installationScan struct {
	Installations   []model.RuntimeInstallation
	HostState       model.FactState
	HostReason      string
	ContainerState  model.FactState
	ContainerReason string
}

func discoverInstallations(parent context.Context, fileSystem fsx.FileSystem, home string, environment map[string]string, processes model.ProcessState, containers model.ContainerState) installationScan {
	if fileSystem == nil {
		return installationScan{HostState: model.StateUnknown, ContainerState: model.StateUnknown}
	}
	ctx, cancel := context.WithTimeout(parent, installationScanBudget)
	defer cancel()

	prefixes := hostPrefixes(home, environment)
	for _, process := range processes.Processes {
		if process.ContainerID == "" || process.PID <= 0 {
			continue
		}
		root := "/proc/" + intString(process.PID) + "/root"
		prefixes = append(prefixes, containerPrefixes(root, process.ContainerID)...)
	}
	for _, container := range containers.Devices {
		if container.ID == "" || container.InitPID <= 0 {
			continue
		}
		root := "/proc/" + intString(container.InitPID) + "/root"
		prefixes = append(prefixes, containerPrefixes(root, container.ID)...)
	}
	prefixes = uniquePrefixes(prefixes)

	result := installationScan{HostState: model.StateNotDetected, ContainerState: model.StateNotDetected}
	seen := map[string]bool{}
	entries := 0
	environments := 0
	hostDenied, containerDenied, incomplete := false, false, false
	for _, prefix := range prefixes {
		if ctx.Err() != nil || entries >= maxDirectoryEntries || environments >= maxEnvironments {
			incomplete = true
			break
		}
		paths, denied := environmentPrefixes(ctx, fileSystem, prefix, &entries)
		if denied {
			if prefix.Scope == "container" {
				containerDenied = true
			} else {
				hostDenied = true
			}
		}
		for _, environmentPath := range paths {
			if ctx.Err() != nil || entries >= maxDirectoryEntries || environments >= maxEnvironments {
				incomplete = true
				break
			}
			environments++
			items, denied := inspectEnvironment(ctx, fileSystem, prefix, environmentPath, &entries)
			if denied {
				if prefix.Scope == "container" {
					containerDenied = true
				} else {
					hostDenied = true
				}
			}
			for _, item := range items {
				key := item.Product + "\x00" + item.Scope + "\x00" + item.ContainerID + "\x00" + item.Path
				if !seen[key] {
					seen[key] = true
					result.Installations = append(result.Installations, item)
				}
			}
		}
	}
	if ctx.Err() != nil {
		incomplete = true
	}
	result.HostState = scanState(false, hostDenied, incomplete)
	containerIncomplete := containers.DaemonState == model.StateUnknown || containers.DaemonState == model.StateTimeout || containers.DaemonState == model.StateParseError
	result.ContainerState = scanState(false, containerDenied || containers.DaemonState == model.StatePermissionDenied, incomplete || containerIncomplete)
	result.HostReason = installationScopeReason("host", result.HostState, incomplete)
	result.ContainerReason = installationScopeReason("running-container", result.ContainerState, incomplete || containerIncomplete)
	sort.Slice(result.Installations, func(i, j int) bool {
		left, right := result.Installations[i], result.Installations[j]
		if left.Product != right.Product {
			return left.Product < right.Product
		}
		if left.Scope != right.Scope {
			return left.Scope < right.Scope
		}
		if left.ContainerID != right.ContainerID {
			return left.ContainerID < right.ContainerID
		}
		return left.Path < right.Path
	})
	return result
}

func installationScopeReason(scope string, state model.FactState, incomplete bool) string {
	switch state {
	case model.StatePermissionDenied:
		return scope + " package paths were permission-blocked; absence was not inferred"
	case model.StateUnknown:
		if incomplete {
			return "bounded " + scope + " package scan reached its time, environment, or directory-entry safety limit; absence was not inferred"
		}
		return scope + " package visibility could not be determined"
	case model.StateNotDetected:
		return "all selected " + scope + " package paths were inspected and no matching metadata was found"
	default:
		return ""
	}
}

func scanState(found, denied, incomplete bool) model.FactState {
	if found {
		return model.StateAvailable
	}
	if denied {
		return model.StatePermissionDenied
	}
	if incomplete {
		return model.StateUnknown
	}
	return model.StateNotDetected
}

func hostPrefixes(home string, environment map[string]string) []searchPrefix {
	values := []searchPrefix{}
	add := func(value string, children bool) {
		value = path.Clean(strings.TrimSpace(value))
		if value != "." && strings.HasPrefix(value, "/") {
			values = append(values, searchPrefix{Path: value, LogicalRoot: value, Scope: "host", Children: children})
		}
	}
	add(environment["CONDA_PREFIX"], false)
	add(environment["VIRTUAL_ENV"], false)
	for _, entry := range strings.Split(environment["PATH"], ":") {
		if path.Base(entry) == "bin" {
			add(path.Dir(entry), false)
		}
	}
	if home != "" {
		add(path.Join(home, ".local"), false)
		for _, name := range []string{"miniconda3", "anaconda3", "miniforge3", "mambaforge", ".conda"} {
			add(path.Join(home, name), false)
		}
	}
	add("/opt/conda", false)
	add("/opt/venv", true)
	add("/usr/local", false)
	add("/usr", false)
	return uniquePrefixes(values)
}

func containerPrefixes(root, id string) []searchPrefix {
	result := []searchPrefix{}
	for _, logical := range []string{"/usr", "/usr/local", "/opt/conda"} {
		result = append(result, searchPrefix{Path: path.Join(root, logical), LogicalRoot: logical, Scope: "container", ContainerID: id})
	}
	result = append(result, searchPrefix{Path: path.Join(root, "/opt/venv"), LogicalRoot: "/opt/venv", Scope: "container", ContainerID: id, Children: true})
	return result
}

func uniquePrefixes(values []searchPrefix) []searchPrefix {
	seen := map[string]bool{}
	result := []searchPrefix{}
	for _, value := range values {
		key := value.Scope + "\x00" + value.ContainerID + "\x00" + value.Path
		if value.Path == "." || seen[key] {
			continue
		}
		seen[key] = true
		result = append(result, value)
	}
	return result
}

func environmentPrefixes(ctx context.Context, fileSystem fsx.FileSystem, prefix searchPrefix, entries *int) ([]string, bool) {
	result := []string{prefix.Path}
	denied := false
	for _, directory := range []string{"envs"} {
		children, err := readDir(ctx, fileSystem, path.Join(prefix.Path, directory), entries)
		if err != nil {
			denied = denied || isPermission(err)
			continue
		}
		for _, child := range children {
			if child.IsDir() {
				result = append(result, path.Join(prefix.Path, directory, child.Name()))
			}
		}
	}
	if prefix.Children {
		children, err := readDir(ctx, fileSystem, prefix.Path, entries)
		if err != nil {
			denied = denied || isPermission(err)
		} else {
			for _, child := range children {
				if child.IsDir() {
					result = append(result, path.Join(prefix.Path, child.Name()))
				}
			}
		}
	}
	return result, denied
}

func inspectEnvironment(ctx context.Context, fileSystem fsx.FileSystem, prefix searchPrefix, environmentPath string, entries *int) ([]model.RuntimeInstallation, bool) {
	result := []model.RuntimeInstallation{}
	denied := false
	for _, lib := range []string{"lib", "lib64"} {
		pythonDirs, err := readDir(ctx, fileSystem, path.Join(environmentPath, lib), entries)
		if err != nil {
			denied = denied || isPermission(err)
			continue
		}
		for _, pythonDir := range pythonDirs {
			if !pythonDir.IsDir() || !strings.HasPrefix(strings.ToLower(pythonDir.Name()), "python") {
				continue
			}
			for _, packages := range []string{"site-packages", "dist-packages"} {
				site := path.Join(environmentPath, lib, pythonDir.Name(), packages)
				items, err := readDir(ctx, fileSystem, site, entries)
				if err != nil {
					denied = denied || isPermission(err)
					continue
				}
				for _, item := range items {
					if !item.IsDir() || !candidateMetadataDir(item.Name()) {
						continue
					}
					metadataPath := path.Join(site, item.Name(), "METADATA")
					installation, ok := readMetadata(fileSystem, metadataPath)
					if !ok {
						continue
					}
					installation.Path = logicalPath(prefix, path.Join(site, item.Name()))
					installation.PythonEnvironment = logicalPath(prefix, environmentPath)
					installation.Scope = prefix.Scope
					installation.ContainerID = prefix.ContainerID
					installation.Source = "python package metadata"
					installation.Confidence = model.ConfidenceHigh
					result = append(result, installation)
				}
			}
		}
	}
	return result, denied
}

func candidateMetadataDir(value string) bool {
	value = strings.ToLower(strings.ReplaceAll(value, "_", "-"))
	if !strings.HasSuffix(value, ".dist-info") {
		return false
	}
	return strings.HasPrefix(value, "vllm-") || strings.HasPrefix(value, "sglang-") || strings.HasPrefix(value, "torch-") || strings.HasPrefix(value, "pytorch-")
}

func readMetadata(fileSystem fsx.FileSystem, name string) (model.RuntimeInstallation, bool) {
	info, err := fileSystem.Stat(name)
	if err != nil || info.Size() < 0 || info.Size() > maxMetadataBytes {
		return model.RuntimeInstallation{}, false
	}
	raw, err := fileSystem.ReadFile(name)
	if err != nil || len(raw) > maxMetadataBytes {
		return model.RuntimeInstallation{}, false
	}
	product, version := "", ""
	for _, line := range strings.Split(string(raw), "\n") {
		key, value, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		switch strings.ToLower(strings.TrimSpace(key)) {
		case "name":
			product = canonicalProduct(strings.TrimSpace(value))
		case "version":
			version = strings.TrimSpace(value)
		}
		if product != "" && version != "" {
			break
		}
	}
	if product == "" {
		return model.RuntimeInstallation{}, false
	}
	return model.RuntimeInstallation{Product: product, Version: version}, true
}

func canonicalProduct(value string) string {
	value = strings.ToLower(strings.ReplaceAll(value, "_", "-"))
	switch value {
	case "torch", "pytorch":
		return "pytorch"
	case "vllm":
		return "vllm"
	case "sglang", "sgl-kernel":
		if value == "sglang" {
			return "sglang"
		}
	}
	return ""
}

func logicalPath(prefix searchPrefix, value string) string {
	if prefix.Scope != "container" {
		return value
	}
	rel := strings.TrimPrefix(value, strings.TrimSuffix(prefix.Path, prefix.LogicalRoot))
	if strings.HasPrefix(rel, "/") {
		return path.Clean(rel)
	}
	return value
}

func readDir(ctx context.Context, fileSystem fsx.FileSystem, name string, entries *int) ([]fs.DirEntry, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	items, err := fileSystem.ReadDir(name)
	if err != nil {
		return nil, err
	}
	*entries += len(items)
	if *entries > maxDirectoryEntries {
		return nil, context.DeadlineExceeded
	}
	return items, nil
}

func isPermission(err error) bool {
	return err != nil && (strings.Contains(strings.ToLower(err.Error()), "permission") || strings.Contains(strings.ToLower(err.Error()), "not permitted"))
}

func intString(value int) string {
	if value == 0 {
		return "0"
	}
	digits := make([]byte, 0, 12)
	for value > 0 {
		digits = append(digits, byte('0'+value%10))
		value /= 10
	}
	for left, right := 0, len(digits)-1; left < right; left, right = left+1, right-1 {
		digits[left], digits[right] = digits[right], digits[left]
	}
	return string(digits)
}
