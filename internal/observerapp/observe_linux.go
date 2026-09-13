package observerapp

import (
	"context"
	"strconv"
	"strings"
	"time"

	"github.com/Monska85/hostlens/internal/dockerobs"
)

// observationBudget bounds one upstream observation regardless of the IPC
// request deadline. Tests shorten it to exercise abandonment recovery.
var observationBudget = 45 * time.Second

func observeContext(ctx context.Context, d time.Duration) (context.Context, context.CancelFunc) {
	return context.WithTimeout(ctx, d)
}

func unixTimePtr(seconds int64) *time.Time {
	if seconds <= 0 {
		return nil
	}
	t := time.Unix(seconds, 0)
	return &t
}

func mountRefs(mounts []daemonMount) []dockerobs.MountRef {
	out := make([]dockerobs.MountRef, 0, len(mounts))
	for _, m := range mounts {
		ref := dockerobs.MountRef{Type: m.Type, Destination: m.Destination, Source: m.Source, RW: m.RW, VolumeName: m.Name}
		if ref.Type == "volume" && ref.Source == "" {
			ref.Source = m.Name
		}
		out = append(out, ref)
	}
	return out
}

func portSummary(ports []struct {
	IP          string `json:"IP"`
	PrivatePort uint16 `json:"PrivatePort"`
	PublicPort  uint16 `json:"PublicPort"`
	Type        string `json:"Type"`
}) []dockerobs.PortMapping {
	out := make([]dockerobs.PortMapping, 0, len(ports))
	for _, p := range ports {
		out = append(out, dockerobs.PortMapping{
			PrivatePort: p.PrivatePort, PublicPort: p.PublicPort, Type: p.Type, HostIP: p.IP,
		})
	}
	return out
}

// engineInfo projects selected daemon facts from one fixed observation.
func (c *client) engineInfo(ctx context.Context) dockerobs.Response {
	info := dockerobs.EngineInfo{MinAPI: c.minAPI, NegotiatedAPI: c.negotiated}
	data, err := c.getTyped(ctx, dockerobs.Request{Version: dockerobs.ProtocolVersion, Operation: dockerobs.OpEngineInfo})
	if err != nil {
		return fail(err)
	}
	var daemon struct {
		ServerVersion     string   `json:"ServerVersion"`
		Os                string   `json:"Os"`
		OSType            string   `json:"OSType"`
		Architecture      string   `json:"Architecture"`
		KernelVersion     string   `json:"KernelVersion"`
		Driver            string   `json:"Driver"`
		CgroupDriver      string   `json:"CgroupDriver"`
		CgroupVersion     string   `json:"CgroupVersion"`
		LoggingDriver     string   `json:"LoggingDriver"`
		ExperimentalBuild bool     `json:"ExperimentalBuild"`
		Containers        int      `json:"Containers"`
		ContainersRunning int      `json:"ContainersRunning"`
		ContainersPaused  int      `json:"ContainersPaused"`
		ContainersStopped int      `json:"ContainersStopped"`
		Images            int      `json:"Images"`
		OperatingSystem   string   `json:"OperatingSystem"`
		SecurityOptions   []string `json:"SecurityOptions"`
		Plugins           struct {
			Log []string `json:"Log"`
		} `json:"Plugins"`
	}
	if e := decodeObject(data, &daemon); e != nil {
		return fail(e)
	}
	info.ServerVersion = daemon.ServerVersion
	info.OperatingSystem = daemon.OperatingSystem
	info.OSType = daemon.OSType
	info.Architecture = daemon.Architecture
	info.KernelVersion = daemon.KernelVersion
	info.StorageDriver = daemon.Driver
	info.CgroupDriver = daemon.CgroupDriver
	info.CgroupVersion = daemon.CgroupVersion
	info.LoggingDriver = daemon.LoggingDriver
	info.Experimental = daemon.ExperimentalBuild
	info.ContainerCount = daemon.Containers
	info.RunningCount = daemon.ContainersRunning
	info.PausedCount = daemon.ContainersPaused
	info.StoppedCount = daemon.ContainersStopped
	info.ImageCount = daemon.Images
	info.SupportedDrivers = daemon.Plugins.Log
	for _, option := range daemon.SecurityOptions {
		if option == "name=rootless" {
			info.UnsupportedReasons = append(info.UnsupportedReasons, "rootless engine")
		}
	}
	if strings.Contains(strings.ToLower(daemon.OperatingSystem), "docker desktop") {
		info.UnsupportedReasons = append(info.UnsupportedReasons, "docker desktop")
	}
	return dockerobs.Response{Engine: &info}
}

func (c *client) containerList(ctx context.Context) dockerobs.Response {
	data, err := c.getTyped(ctx, dockerobs.Request{Version: dockerobs.ProtocolVersion, Operation: dockerobs.OpContainerList})
	if err != nil {
		return notFound(err)
	}
	items, truncated, err := decodeList[daemonContainerSummary](data, dockerobs.MaxContainerItems)
	if err != nil {
		return fail(err)
	}
	list := make([]dockerobs.ContainerSummary, 0, len(items))
	networks := map[string]bool{}
	for _, s := range items {
		summary := dockerobs.ContainerSummary{
			ID: s.ID, Image: s.Image, ImageID: s.ImageID, State: s.State, Status: s.Status,
			Created: unixTimePtr(s.Created),
			Ports:   portSummary(s.Ports),
			Mounts:  mountRefs(s.Mounts),
		}
		for _, raw := range s.Names {
			summary.Names = append(summary.Names, strings.TrimPrefix(raw, "/"))
		}
		for name, endpoint := range s.NetworkSettings.Networks {
			// The list endpoint keys attachments by network name; some
			// daemons omit the value's Name field.
			attached := endpoint.Name
			if attached == "" {
				attached = name
			}
			if attached != "" && !networks[attached] {
				networks[attached] = true
				summary.Networks = append(summary.Networks, attached)
			}
		}
		list = append(list, summary)
	}
	return dockerobs.Response{Containers: list, Truncated: truncated}
}

func (c *client) containerDetail(ctx context.Context, r dockerobs.Request) dockerobs.Response {
	data, err := c.getTyped(ctx, r)
	if err != nil {
		return notFound(err)
	}
	var d daemonContainerInspect
	if e := decodeObject(data, &d); e != nil {
		return fail(e)
	}
	summary := dockerobs.ContainerSummary{
		ID:            d.ID,
		Image:         d.Config.Image,
		ImageID:       d.Image,
		State:         d.State.Status,
		Status:        d.State.Status,
		Health:        d.State.Health.Status,
		Created:       parseDockerTime(d.Created),
		Names:         []string{strings.TrimPrefix(d.Name, "/")},
		Mounts:        mountRefs(d.Mounts),
		LogDriver:     d.HostConfig.LogConfig.Type,
		RestartPolicy: d.HostConfig.RestartPolicy.Name,
		RestartCount:  &d.RestartCount,
		PidsLimit:     d.HostConfig.PidsLimit,
		NanoCPUs:      d.HostConfig.NanoCpus,
		CPUShares:     d.HostConfig.CPUShares,
		CPUPeriod:     d.HostConfig.CPUPeriod,
		CPUQuota:      d.HostConfig.CPUQuota,
		Memory:        d.HostConfig.Memory,
		MemorySwap:    d.HostConfig.MemorySwap,
		StartedAt:     parseDockerTime(d.State.StartedAt),
		FinishedAt:    parseDockerTime(d.State.FinishedAt),
	}
	for key, bindings := range d.NetworkSettings.Ports {
		private, proto, ok := strings.Cut(key, "/")
		if !ok {
			continue
		}
		privatePort64, e := strconv.ParseUint(private, 10, 16)
		if e != nil {
			continue
		}
		privatePort := uint16(privatePort64)
		for _, binding := range bindings {
			public, e := strconv.ParseUint(binding.HostPort, 10, 16)
			if e != nil {
				continue
			}
			summary.Ports = append(summary.Ports, dockerobs.PortMapping{
				PrivatePort: privatePort, PublicPort: uint16(public), Type: proto, HostIP: binding.HostIP,
			})
		}
	}
	detail := dockerobs.ContainerDetail{ContainerSummary: summary}
	detail.OomKilled = d.State.OOMKilled
	detail.Pid = &d.State.Pid
	detail.NetworkMode = d.HostConfig.NetworkMode
	if d.Config.Healthcheck != nil && len(d.Config.Healthcheck.Test) > 0 {
		// Only the fact that a health check exists is observable; its
		// command, arguments, and output never cross the boundary.
		detail.HealthCheck = "configured"
	}
	if d.State.Health.FailingStreak > 0 {
		detail.HealthFailing = &d.State.Health.FailingStreak
	}
	for name, endpoint := range d.NetworkSettings.Networks {
		detail.Endpoints = append(detail.Endpoints, dockerobs.Endpoint{
			Name: name, IPAddress: endpoint.IPAddress, MACAddress: endpoint.MacAddress,
		})
	}
	return dockerobs.Response{Detail: &detail}
}

func (c *client) containerStats(ctx context.Context, r dockerobs.Request) dockerobs.Response {
	data, err := c.getTyped(ctx, r)
	if err != nil {
		return notFound(err)
	}
	var s daemonStats
	if e := decodeObject(data, &s); e != nil {
		return fail(e)
	}
	out := dockerobs.ContainerStats{}
	if read, e := time.Parse(time.RFC3339Nano, s.Read); e == nil {
		out.Read = read
	}
	if pre, e := time.Parse(time.RFC3339Nano, s.PreRead); e == nil && !pre.IsZero() {
		out.Preread = pre
	}
	out.OnlineCPUs = s.CPUStats.OnlineCPUs
	out.CPUUsage = &s.CPUStats.CPUUsage.TotalUsage
	out.SystemUsage = s.CPUStats.SystemUsage
	out.PreCPUUsage = &s.PreCPUStats.CPUUsage.TotalUsage
	out.PreSystem = s.PreCPUStats.SystemUsage
	if *out.CPUUsage > *out.PreCPUUsage && out.SystemUsage != nil && out.PreSystem != nil && *out.SystemUsage > *out.PreSystem {
		online := 1
		if s.CPUStats.OnlineCPUs != nil && *s.CPUStats.OnlineCPUs > 0 {
			online = *s.CPUStats.OnlineCPUs
		}
		percent := float64(*out.CPUUsage-*out.PreCPUUsage) / float64(*out.SystemUsage-*out.PreSystem) * float64(online) * 100
		out.CPUPercent = &percent
	}
	out.MemoryUsed = s.MemoryStats.Usage
	out.MemoryLimit = s.MemoryStats.Limit
	cache := s.MemoryStats.Stats.Cache
	if cache == 0 {
		cache = s.MemoryStats.Stats.InactiveFile
	}
	out.MemoryCache = &cache
	for _, entry := range s.BlkIOStats.IoServiceBytesRecursive {
		switch entry.Op {
		case "Read", "read":
			if out.BlockRead == nil {
				out.BlockRead = &entry.Value
			} else {
				*out.BlockRead += entry.Value
			}
		case "Write", "write":
			if out.BlockWrite == nil {
				out.BlockWrite = &entry.Value
			} else {
				*out.BlockWrite += entry.Value
			}
		}
	}
	if len(s.Networks) > 0 {
		out.Networks = make(map[string]dockerobs.NetIO, len(s.Networks))
		for name, n := range s.Networks {
			out.Networks[name] = dockerobs.NetIO{RxBytes: n.RxBytes, TxBytes: n.TxBytes}
		}
	}
	out.PidsCurrent = s.PidsStats.Current
	out.PidsLimit = s.PidsStats.Limit
	return dockerobs.Response{Stats: &out}
}

func (c *client) imageList(ctx context.Context, r dockerobs.Request) dockerobs.Response {
	data, err := c.getTyped(ctx, r)
	if err != nil {
		return notFound(err)
	}
	items, truncated, err := decodeList[daemonImage](data, dockerobs.MaxImageItems)
	if err != nil {
		return fail(err)
	}
	out := make([]dockerobs.ImageSummary, 0, len(items))
	for _, d := range items {
		summary := dockerobs.ImageSummary{
			ID: d.ID, RepoTags: d.RepoTags, RepoDigests: d.RepoDigests,
			Created: unixTimePtr(d.Created),
		}
		if d.Size != 0 {
			summary.Size = &d.Size
		}
		if d.SharedSize > 0 {
			summary.SharedSize = &d.SharedSize
		}
		if d.Containers > 0 {
			summary.ContainerRefs = &d.Containers
		}
		if len(d.RepoTags) == 0 && len(d.RepoDigests) == 0 {
			summary.Dangling = true
		}
		out = append(out, summary)
	}
	return dockerobs.Response{Images: out, Truncated: truncated}
}

func (c *client) volumeList(ctx context.Context) dockerobs.Response {
	data, err := c.getTyped(ctx, dockerobs.Request{Version: dockerobs.ProtocolVersion, Operation: dockerobs.OpVolumeList})
	if err != nil {
		return notFound(err)
	}
	var payload struct {
		Volumes []daemonVolume `json:"Volumes"`
	}
	if e := decodeObject(data, &payload); e != nil {
		return fail(e)
	}
	truncated := false
	if len(payload.Volumes) > dockerobs.MaxVolumeItems {
		payload.Volumes = payload.Volumes[:dockerobs.MaxVolumeItems]
		truncated = true
	}
	out := make([]dockerobs.VolumeSummary, 0, len(payload.Volumes))
	for _, d := range payload.Volumes {
		out = append(out, dockerobs.VolumeSummary{
			Name:      d.Name,
			Driver:    d.Driver,
			Scope:     d.Scope,
			Created:   parseDockerTime(d.CreatedAt),
			Anonymous: d.Labels["com.docker.volume.anonymous"] == "true",
		})
	}
	return dockerobs.Response{Volumes: out, Truncated: truncated}
}

func (c *client) networkList(ctx context.Context) dockerobs.Response {
	data, err := c.getTyped(ctx, dockerobs.Request{Version: dockerobs.ProtocolVersion, Operation: dockerobs.OpNetworkList})
	if err != nil {
		return notFound(err)
	}
	items, truncated, err := decodeList[daemonNetwork](data, dockerobs.MaxNetworkItems)
	if err != nil {
		return fail(err)
	}
	out := make([]dockerobs.NetworkSummary, 0, len(items))
	for _, d := range items {
		summary := dockerobs.NetworkSummary{
			ID: d.ID, Name: d.Name, Driver: d.Driver, Scope: d.Scope,
			Internal: d.Internal, EnableIPv6: d.IPv6,
		}
		for _, cfg := range d.IPAM.Config {
			if cfg.Subnet != "" {
				summary.Subnets = append(summary.Subnets, cfg.Subnet)
			}
		}
		out = append(out, summary)
	}
	return dockerobs.Response{Networks: out, Truncated: truncated}
}

func (c *client) diskUsage(ctx context.Context) dockerobs.Response {
	data, err := c.getTyped(ctx, dockerobs.Request{Version: dockerobs.ProtocolVersion, Operation: dockerobs.OpDiskUsage})
	if err != nil {
		return notFound(err)
	}
	var d daemonDiskUsage
	if e := decodeObject(data, &d); e != nil {
		return fail(e)
	}
	out := dockerobs.DiskUsage{LayersSize: &d.LayersSize}
	for _, image := range d.Images {
		summary := dockerobs.ImageSummary{
			ID: image.ID, RepoTags: image.RepoTags, RepoDigests: image.RepoDigests,
			Created: unixTimePtr(image.Created),
		}
		if image.Size != 0 {
			summary.Size = &image.Size
		}
		if image.SharedSize > 0 {
			summary.SharedSize = &image.SharedSize
		}
		out.Images = append(out.Images, summary)
	}
	for _, container := range d.Containers {
		ref := dockerobs.ContainerRef{ID: container.ID}
		if container.SizeRootFs != 0 {
			ref.SizeRootF = &container.SizeRootFs
		}
		if container.SizeRw != 0 {
			ref.SizeRw = &container.SizeRw
		}
		out.Containers = append(out.Containers, ref)
	}
	for _, volume := range d.Volumes {
		summary := dockerobs.VolumeSummary{
			Name: volume.Name, Driver: volume.Driver, Created: parseDockerTime(volume.CreatedAt),
		}
		if volume.UsageData != nil {
			if volume.UsageData.RefCount > 0 {
				summary.RefCount = &volume.UsageData.RefCount
			}
			if volume.UsageData.Size > 0 {
				summary.Size = &volume.UsageData.Size
			}
		}
		out.Volumes = append(out.Volumes, summary)
	}
	if len(d.BuildCache) > 0 {
		var total int64
		for _, cache := range d.BuildCache {
			if cache.Size > 0 {
				total += cache.Size
			}
		}
		out.BuildCacheSize = &total
		count := len(d.BuildCache)
		out.BuildCacheItems = &count
	}
	return dockerobs.Response{Usage: &out}
}
