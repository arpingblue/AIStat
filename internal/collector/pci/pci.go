package pci

import (
	"context"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/arpingblue/AIStat/internal/collector"
	"github.com/arpingblue/AIStat/internal/execx"
	"github.com/arpingblue/AIStat/internal/model"
)

type Collector struct{}

func (Collector) ID() collector.ID                 { return "pci" }
func (Collector) Provides() []collector.Capability { return []collector.Capability{"pci"} }
func (Collector) Requires() []collector.Capability { return []collector.Capability{"numa"} }

func (c Collector) Collect(ctx context.Context, env collector.Env) collector.Result {
	if env.Platform != "linux" && !env.Fixture {
		return collector.Unsupported(c.ID(), "pci")
	}
	entries, err := env.FileSystem.ReadDir("/sys/bus/pci/devices")
	if err != nil {
		return fail(c.ID(), collector.FileErrorState(err), err)
	}
	devices := []model.PCIDevice{}
	for _, entry := range entries {
		base := "/sys/bus/pci/devices/" + entry.Name()
		device := model.PCIDevice{Address: canonicalBDF(entry.Name())}
		device.Class = readTrim(env, base+"/class")
		device.VendorID = readTrim(env, base+"/vendor")
		device.DeviceID = readTrim(env, base+"/device")
		if value, parseErr := strconv.Atoi(readTrim(env, base+"/numa_node")); parseErr == nil && value >= 0 {
			device.NUMANode = model.Int(value)
		}
		device.LinkWidth, _ = strconv.Atoi(readTrim(env, base+"/current_link_width"))
		device.MaxWidth, _ = strconv.Atoi(readTrim(env, base+"/max_link_width"))
		device.LinkSpeedGT = parseSpeed(readTrim(env, base+"/current_link_speed"))
		device.MaxSpeedGT = parseSpeed(readTrim(env, base+"/max_link_speed"))
		if link, err := env.FileSystem.Readlink(base + "/iommu_group"); err == nil {
			device.IOMMUGroup = filepath.Base(link)
		}
		if link, err := env.FileSystem.Readlink(base + "/driver"); err == nil {
			device.Driver = filepath.Base(link)
		}
		if link, err := env.FileSystem.Readlink(base); err == nil {
			device.Parent = parentBDF(link, device.Address)
		}
		devices = append(devices, device)
	}
	if env.Runner != nil {
		if result, runErr := env.Runner.Run(ctx, execx.CommandSpec{Name: "lspci", Args: []string{"-D", "-vv"}, Timeout: 2 * time.Second, OutputLimit: 4 << 20}); runErr == nil {
			acs := ParseACS(result.Stdout)
			for i := range devices {
				if value, ok := acs[strings.ToLower(devices[i].Address)]; ok {
					devices[i].ACSRedirect = &value
				}
			}
		}
	}
	value := model.PCIState{State: model.StateAvailable, Devices: devices}
	return collector.Result{Collector: c.ID(), State: model.StateAvailable, Facts: []model.Fact{model.NewFact("pci", model.StateAvailable, value, model.ConfidenceHigh, model.SourceRef{Collector: "pci", Source: "/sys/bus/pci/devices"})}}
}

func canonicalBDF(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	if strings.Count(value, ":") == 1 {
		return "0000:" + value
	}
	parts := strings.Split(value, ":")
	if len(parts) == 3 && len(parts[0]) > 4 {
		parts[0] = parts[0][len(parts[0])-4:]
		return strings.Join(parts, ":")
	}
	return value
}

func ParseACS(raw string) map[string]bool {
	result := map[string]bool{}
	current := ""
	for _, line := range strings.Split(strings.ReplaceAll(raw, "\r\n", "\n"), "\n") {
		if len(line) > 0 && line[0] != ' ' && line[0] != '\t' {
			fields := strings.Fields(line)
			if len(fields) > 0 && strings.Count(fields[0], ":") == 2 && strings.Contains(fields[0], ".") {
				current = strings.ToLower(fields[0])
			} else {
				current = ""
			}
			continue
		}
		if current != "" && strings.Contains(line, "ACSCtl:") {
			control := strings.ToLower(line)
			result[current] = strings.Contains(control, "reqredir+") || strings.Contains(control, "cmpltredir+") || strings.Contains(control, "upstreamfwd+")
		}
	}
	return result
}

func readTrim(env collector.Env, path string) string {
	raw, err := env.FileSystem.ReadFile(path)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(raw))
}
func parseSpeed(raw string) float64 {
	field := strings.Fields(raw)
	if len(field) == 0 {
		return 0
	}
	value, _ := strconv.ParseFloat(field[0], 64)
	return value
}
func parentBDF(link, self string) string {
	parts := strings.FieldsFunc(filepath.ToSlash(link), func(r rune) bool { return r == '/' })
	for i := len(parts) - 1; i >= 0; i-- {
		p := strings.ToLower(parts[i])
		if p != strings.ToLower(self) && strings.Count(p, ":") == 2 && strings.Contains(p, ".") {
			return p
		}
	}
	return ""
}
func fail(id collector.ID, state model.FactState, err error) collector.Result {
	return collector.Result{Collector: id, State: state, Facts: []model.Fact{{Key: "pci", State: state, Confidence: model.ConfidenceLow}}, Diagnostics: []model.Diagnostic{{Level: "warn", Code: "read_or_parse", Message: err.Error()}}}
}
