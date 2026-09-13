package observerapp

// Daemon response structures decode upstream JSON. Unknown future fields are
// ignored during decode and never projected. Nothing here is exported: raw
// daemon objects cannot cross the observer IPC boundary.

type daemonContainerSummary struct {
	ID      string   `json:"Id"`
	Names   []string `json:"Names"`
	Image   string   `json:"Image"`
	ImageID string   `json:"ImageID"`
	State   string   `json:"State"`
	Status  string   `json:"Status"`
	// Created is the daemon's list timestamp in Unix seconds.
	Created int64 `json:"Created"`
	Ports   []struct {
		IP          string `json:"IP"`
		PrivatePort uint16 `json:"PrivatePort"`
		PublicPort  uint16 `json:"PublicPort"`
		Type        string `json:"Type"`
	} `json:"Ports"`
	Mounts          []daemonMount `json:"Mounts"`
	NetworkSettings struct {
		Networks map[string]struct {
			Name string `json:"Name"`
		} `json:"Networks"`
	} `json:"NetworkSettings"`
}

type daemonMount struct {
	Type        string `json:"Type"`
	Name        string `json:"Name"`
	Source      string `json:"Source"`
	Destination string `json:"Destination"`
	RW          bool   `json:"RW"`
}

type daemonPortBinding struct {
	HostIP string `json:"HostIp"`
	// HostPort is a JSON string in the Engine API inspect response.
	HostPort string `json:"HostPort"`
}

type daemonContainerInspect struct {
	ID      string `json:"Id"`
	Created string `json:"Created"`
	Name    string `json:"Name"`
	Image   string `json:"Image"`
	Config  struct {
		Image       string `json:"Image"`
		Healthcheck *struct {
			Test []string `json:"Test"`
		} `json:"Healthcheck"`
	} `json:"Config"`
	State struct {
		Status     string `json:"Status"`
		OOMKilled  bool   `json:"OOMKilled"`
		Pid        int    `json:"Pid"`
		StartedAt  string `json:"StartedAt"`
		FinishedAt string `json:"FinishedAt"`
		Health     struct {
			Status        string `json:"Status"`
			FailingStreak int    `json:"FailingStreak"`
			// Health log entries are decoded into memory only; their
			// command output never reaches any projection.
			Log []struct {
				Start    string `json:"Start"`
				End      string `json:"End"`
				ExitCode int    `json:"ExitCode"`
				Output   string `json:"Output"`
			} `json:"Log"`
		} `json:"Health"`
	} `json:"State"`
	RestartCount int `json:"RestartCount"`
	HostConfig   struct {
		LogConfig struct {
			Type   string            `json:"Type"`
			Config map[string]string `json:"Config"`
		} `json:"LogConfig"`
		RestartPolicy struct {
			Name string `json:"Name"`
		} `json:"RestartPolicy"`
		NetworkMode string `json:"NetworkMode"`
		PidsLimit   *int64 `json:"PidsLimit"`
		NanoCpus    *int64 `json:"NanoCpus"`
		CPUShares   *int64 `json:"CpuShares"`
		CPUPeriod   *int64 `json:"CpuPeriod"`
		CPUQuota    *int64 `json:"CpuQuota"`
		Memory      *int64 `json:"Memory"`
		MemorySwap  *int64 `json:"MemorySwap"`
	} `json:"HostConfig"`
	NetworkSettings struct {
		Ports    map[string][]daemonPortBinding `json:"Ports"`
		Networks map[string]struct {
			IPAddress  string `json:"IPAddress"`
			MacAddress string `json:"MacAddress"`
		} `json:"Networks"`
	} `json:"NetworkSettings"`
	Mounts []daemonMount `json:"Mounts"`
}

type daemonStats struct {
	Read     string `json:"read"`
	PreRead  string `json:"preread"`
	CPUStats struct {
		CPUUsage struct {
			TotalUsage uint64 `json:"total_usage"`
		} `json:"cpu_usage"`
		SystemUsage *uint64 `json:"system_cpu_usage"`
		OnlineCPUs  *int    `json:"online_cpus"`
	} `json:"cpu_stats"`
	PreCPUStats struct {
		CPUUsage struct {
			TotalUsage uint64 `json:"total_usage"`
		} `json:"cpu_usage"`
		SystemUsage *uint64 `json:"system_cpu_usage"`
	} `json:"precpu_stats"`
	MemoryStats struct {
		Usage *uint64 `json:"usage"`
		Limit *uint64 `json:"limit"`
		Stats struct {
			Cache        uint64 `json:"cache"`
			InactiveFile uint64 `json:"inactive_file"`
			ActiveFile   uint64 `json:"active_file"`
		} `json:"stats"`
	} `json:"memory_stats"`
	BlkIOStats struct {
		IoServiceBytesRecursive []struct {
			Op    string `json:"op"`
			Value uint64 `json:"value"`
		} `json:"io_service_bytes_recursive"`
	} `json:"blkio_stats"`
	Networks map[string]struct {
		RxBytes *uint64 `json:"rx_bytes"`
		TxBytes *uint64 `json:"tx_bytes"`
	} `json:"networks"`
	PidsStats struct {
		Current *int `json:"current"`
		Limit   *int `json:"limit"`
	} `json:"pids_stats"`
}

type daemonImage struct {
	ID          string   `json:"Id"`
	RepoTags    []string `json:"RepoTags"`
	RepoDigests []string `json:"RepoDigests"`
	Created     int64    `json:"Created"`
	Size        int64    `json:"Size"`
	SharedSize  int64    `json:"SharedSize"`
	UniqueSize  int64    `json:"UniqueSize"`
	Containers  int      `json:"Containers"`
}

type daemonVolume struct {
	Name      string            `json:"Name"`
	Driver    string            `json:"Driver"`
	Scope     string            `json:"Scope"`
	CreatedAt string            `json:"CreatedAt"`
	Labels    map[string]string `json:"Labels"`
	UsageData *struct {
		RefCount int   `json:"RefCount"`
		Size     int64 `json:"Size"`
	} `json:"UsageData"`
}

type daemonNetwork struct {
	ID       string `json:"Id"`
	Name     string `json:"Name"`
	Driver   string `json:"Driver"`
	Scope    string `json:"Scope"`
	Internal bool   `json:"Internal"`
	IPv6     bool   `json:"EnableIPv6"`
	IPAM     struct {
		Config []struct {
			Subnet string `json:"Subnet"`
		} `json:"Config"`
	} `json:"IPAM"`
	Labels map[string]string `json:"Labels"`
}

type daemonDiskUsage struct {
	LayersSize int64 `json:"LayersSize"`
	Images     []struct {
		ID          string   `json:"Id"`
		RepoTags    []string `json:"RepoTags"`
		RepoDigests []string `json:"RepoDigests"`
		Created     int64    `json:"Created"`
		Size        int64    `json:"Size"`
		SharedSize  int64    `json:"SharedSize"`
	} `json:"Images"`
	Containers []struct {
		ID         string `json:"Id"`
		SizeRootFs int64  `json:"SizeRootFs"`
		SizeRw     int64  `json:"SizeRw"`
	} `json:"Containers"`
	Volumes    []daemonVolume `json:"Volumes"`
	BuildCache []struct {
		Size int64 `json:"Size"`
	} `json:"BuildCache"`
}
