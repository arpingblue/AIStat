package network

import (
	"context"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/arpingblue/AIStat/internal/collector"
	"github.com/arpingblue/AIStat/internal/model"
)

type Collector struct{}

func (Collector) ID() collector.ID                 { return "network" }
func (Collector) Provides() []collector.Capability { return []collector.Capability{"network", "rdma"} }
func (Collector) Requires() []collector.Capability { return []collector.Capability{"pci"} }

func (c Collector) Collect(_ context.Context, env collector.Env) collector.Result {
	if env.Platform != "linux" && !env.Fixture {
		return collector.Result{Collector: c.ID(), State: model.StateUnsupported, Facts: []model.Fact{{Key: "network", State: model.StateUnsupported, Confidence: model.ConfidenceHigh}, {Key: "rdma", State: model.StateUnsupported, Confidence: model.ConfidenceHigh}}}
	}
	netState, netErr := collectNICs(env)
	rdmaState := collectRDMA(env)
	if netErr != nil {
		state := collector.FileErrorState(netErr)
		return collector.Result{Collector: c.ID(), State: state, Facts: []model.Fact{{Key: "network", State: state, Confidence: model.ConfidenceLow}, model.NewFact("rdma", rdmaState.State, rdmaState, model.ConfidenceMedium)}, Diagnostics: []model.Diagnostic{{Level: "warn", Code: "read_or_parse", Message: netErr.Error()}}}
	}
	rdmaFact := model.Fact{Key: "rdma", State: rdmaState.State, Confidence: model.ConfidenceMedium, Sources: []model.SourceRef{{Collector: "network", Source: "/sys/class/infiniband"}}}
	if rdmaState.State == model.StateAvailable {
		rdmaFact = model.NewFact("rdma", model.StateAvailable, rdmaState, model.ConfidenceMedium, model.SourceRef{Collector: "network", Source: "/sys/class/infiniband"})
	}
	return collector.Result{Collector: c.ID(), State: model.StateAvailable, Facts: []model.Fact{model.NewFact("network", model.StateAvailable, netState, model.ConfidenceHigh, model.SourceRef{Collector: "network", Source: "/sys/class/net"}), rdmaFact}}
}

func collectNICs(env collector.Env) (model.NetworkState, error) {
	entries, err := env.FileSystem.ReadDir("/sys/class/net")
	if err != nil {
		return model.NetworkState{}, err
	}
	nics := []model.NIC{}
	for _, entry := range entries {
		base := "/sys/class/net/" + entry.Name()
		nic := model.NIC{Name: entry.Name()}
		nic.OperState = read(env, base+"/operstate")
		nic.MTU, _ = strconv.Atoi(read(env, base+"/mtu"))
		nic.SpeedMbps, _ = strconv.ParseInt(read(env, base+"/speed"), 10, 64)
		if link, err := env.FileSystem.Readlink(base + "/device"); err == nil {
			nic.PCIAddress = strings.ToLower(filepath.Base(link))
		}
		if value, parseErr := strconv.Atoi(read(env, base+"/device/numa_node")); parseErr == nil && value >= 0 {
			nic.NUMANode = model.Int(value)
		}
		if link, err := env.FileSystem.Readlink(base + "/device/driver"); err == nil {
			nic.Driver = filepath.Base(link)
		}
		nics = append(nics, nic)
	}
	return model.NetworkState{State: model.StateAvailable, NICs: nics}, nil
}

func collectRDMA(env collector.Env) model.RDMAState {
	entries, err := env.FileSystem.ReadDir("/sys/class/infiniband")
	if err != nil {
		return model.RDMAState{State: model.StateNotDetected}
	}
	devices := []model.RDMADevice{}
	for _, entry := range entries {
		base := "/sys/class/infiniband/" + entry.Name()
		device := model.RDMADevice{Name: entry.Name()}
		if link, err := env.FileSystem.Readlink(base + "/device"); err == nil {
			device.PCIAddress = strings.ToLower(filepath.Base(link))
		}
		if value, parseErr := strconv.Atoi(read(env, base+"/device/numa_node")); parseErr == nil && value >= 0 {
			device.NUMANode = model.Int(value)
		}
		if netEntries, netErr := env.FileSystem.ReadDir(base + "/device/net"); netErr == nil && len(netEntries) > 0 {
			device.NetDevice = netEntries[0].Name()
		}
		if ports, portsErr := env.FileSystem.ReadDir(base + "/ports"); portsErr == nil && len(ports) > 0 {
			portBase := base + "/ports/" + ports[0].Name()
			device.State = trimRDMAValue(read(env, portBase+"/state"))
			device.LinkLayer = trimRDMAValue(read(env, portBase+"/link_layer"))
		}
		devices = append(devices, device)
	}
	if len(devices) == 0 {
		return model.RDMAState{State: model.StateNotDetected}
	}
	return model.RDMAState{State: model.StateAvailable, Devices: devices}
}
func trimRDMAValue(value string) string {
	value = strings.TrimSpace(value)
	if _, right, ok := strings.Cut(value, ":"); ok {
		return strings.TrimSpace(right)
	}
	return value
}
func read(env collector.Env, path string) string {
	raw, err := env.FileSystem.ReadFile(path)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(raw))
}
