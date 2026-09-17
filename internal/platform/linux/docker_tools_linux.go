package linux

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/Monska85/hostlens/internal/contract"
	"github.com/Monska85/hostlens/internal/dockerobs"
	"github.com/Monska85/hostlens/internal/policy"
)

var errAmbiguous = errors.New("selector is ambiguous")

// resolveContainer matches a selector against one bounded live inventory.
// Full stable identities, unambiguous hexadecimal prefixes, and exact
// current names resolve; anything else fails without choosing a resource.
func resolveContainer(items []dockerobs.ContainerSummary, selector string) (string, *dockerobs.ContainerSummary, error) {
	if dockerobs.ValidID(selector) {
		for i := range items {
			if items[i].ID == selector {
				return items[i].ID, &items[i], nil
			}
		}
		return "", nil, errors.New("container not found")
	}
	var matches []*dockerobs.ContainerSummary
	if len(selector) >= 12 && isHex(selector) {
		for i := range items {
			if strings.HasPrefix(items[i].ID, selector) {
				matches = append(matches, &items[i])
			}
		}
	} else {
		for i := range items {
			for _, name := range items[i].Names {
				if name == selector {
					matches = append(matches, &items[i])
					break
				}
			}
		}
	}
	switch len(matches) {
	case 0:
		return "", nil, errors.New("container not found")
	case 1:
		return matches[0].ID, matches[0], nil
	default:
		return "", nil, errAmbiguous
	}
}

func isHex(s string) bool {
	for _, r := range s {
		switch {
		case r >= '0' && r <= '9', r >= 'a' && r <= 'f', r >= 'A' && r <= 'F':
		default:
			return false
		}
	}
	return true
}

// selectorFailure reports one failed selector resolution honestly.
func selectorFailure(r *contract.Result, selector string, err error) {
	r.Error = true
	code := "not_found"
	switch {
	case errors.Is(err, errAmbiguous):
		code = "ambiguous_selector"
	case strings.Contains(err.Error(), "unsafe"):
		code = "invalid_selector"
	}
	r.Issue(code, selector, err.Error())
}

// deniedResult reports a policy denial for the requested resource class.
func deniedResult(r *contract.Result, kind, selector string) {
	r.Error = true
	r.Issue("policy_denied", "docker", fmt.Sprintf("%s resource %q is denied by policy", kind, selector))
}

// observationFailure mirrors one observer-reported failure.
func observationFailure(r *contract.Result, source string, response dockerobs.Response) {
	r.Error = true
	code := "docker_unavailable"
	if response.Issue == "not_found" {
		code = "not_found"
	}
	r.Issue(code, source, response.Reason)
}

// containerFingerprint bounds the reference basis of one unused analysis so
// a racing daemon change is detected before candidates are derived.
func containerFingerprint(items []dockerobs.ContainerSummary) dockerobs.StableFingerprint {
	f := dockerobs.StableFingerprint{References: map[string]string{}}
	for _, item := range items {
		f.ContainerIDs = append(f.ContainerIDs, item.ID)
		f.References[item.ID] = item.ImageID
		for _, m := range item.Mounts {
			if m.Type == "volume" && m.VolumeName != "" {
				f.References[item.ID+"|volume|"+m.VolumeName] = "1"
			}
		}
		for _, n := range item.Networks {
			f.References[item.ID+"|network|"+n] = "1"
		}
	}
	return f
}

// correlatedInventory observes the all-container inventory around one
// correlated list observation. When the bounded reference fingerprint
// changed, it retries once within the request deadline; persistent
// instability keeps observations but never classifies uncertain resources
// as unused.
func (c *Collector) correlatedInventory(ctx context.Context, operation string) (dockerobs.Response, []dockerobs.ContainerSummary, bool, error) {
	observeContainers := func() (dockerobs.Response, error) {
		return c.observe(ctx, dockerobs.Request{Version: dockerobs.ProtocolVersion, Operation: dockerobs.OpContainerList})
	}
	before, e := observeContainers()
	if e != nil {
		return dockerobs.Response{}, nil, false, e
	}
	if before.Failed {
		return dockerobs.Response{}, nil, false, errors.New(before.Reason)
	}
	response, e := c.observe(ctx, dockerobs.Request{Version: dockerobs.ProtocolVersion, Operation: operation})
	if e != nil {
		return dockerobs.Response{}, nil, false, e
	}
	if response.Failed {
		return dockerobs.Response{}, nil, false, errors.New(response.Reason)
	}
	after, e := observeContainers()
	if e != nil {
		return dockerobs.Response{}, nil, false, e
	}
	if after.Failed {
		return dockerobs.Response{}, nil, false, errors.New(after.Reason)
	}
	stable := containerFingerprint(before.Containers).Equal(containerFingerprint(after.Containers))
	if stable {
		return response, after.Containers, true, nil
	}
	retry, e := c.observe(ctx, dockerobs.Request{Version: dockerobs.ProtocolVersion, Operation: operation})
	if e != nil {
		return dockerobs.Response{}, nil, false, e
	}
	if retry.Failed {
		return dockerobs.Response{}, nil, false, errors.New(retry.Reason)
	}
	afterRetry, e := observeContainers()
	if e != nil {
		return dockerobs.Response{}, nil, false, e
	}
	if afterRetry.Failed {
		return dockerobs.Response{}, nil, false, errors.New(afterRetry.Reason)
	}
	stable = containerFingerprint(after.Containers).Equal(containerFingerprint(afterRetry.Containers))
	return retry, afterRetry.Containers, stable, nil
}

// resolveContainerLive resolves one selector against the live inventory.
func (c *Collector) resolveContainerLive(ctx context.Context, selector string) (string, *dockerobs.ContainerSummary, error) {
	response, e := c.observe(ctx, dockerobs.Request{Version: dockerobs.ProtocolVersion, Operation: dockerobs.OpContainerList})
	if e != nil {
		return "", nil, e
	}
	if response.Failed {
		return "", nil, errors.New(response.Reason)
	}
	return resolveContainer(response.Containers, selector)
}

// recheckContainerIdentity verifies the selector still resolves to the same
// stable identity after an observation, so name reuse mid-request cannot
// release evidence for an unauthorized replacement.
func (c *Collector) recheckContainerIdentity(ctx context.Context, selector, id string) error {
	if dockerobs.ValidID(selector) {
		return nil
	}
	response, e := c.observe(ctx, dockerobs.Request{Version: dockerobs.ProtocolVersion, Operation: dockerobs.OpContainerList})
	if e != nil {
		return fmt.Errorf("identity recheck unavailable: %w", e)
	}
	if response.Failed {
		return fmt.Errorf("identity recheck unavailable: %s", response.Reason)
	}
	current, _, e := resolveContainer(response.Containers, selector)
	if e != nil {
		return fmt.Errorf("selector no longer resolves to the observed identity: %w", e)
	}
	if current != id {
		return errors.New("selector now resolves to a different container identity")
	}
	return nil
}

// imageDenied evaluates one image against its stable identity and every
// practical tag or digest form; denial through any form excludes it.
func imageDenied(p *policy.Policy, image dockerobs.ImageSummary) bool {
	if !p.DockerListDecision("image", firstImageSelector(image), image.ID, "images") {
		return true
	}
	for _, tag := range image.RepoTags {
		if p.DockerDenied("image", tag) {
			return true
		}
	}
	for _, digest := range image.RepoDigests {
		if p.DockerDenied("image", digest) {
			return true
		}
	}
	return false
}

func firstImageSelector(image dockerobs.ImageSummary) string {
	if len(image.RepoTags) > 0 {
		return image.RepoTags[0]
	}
	return ""
}

// containerReferences indexes current references from the live inventory.
func containerReferences(containers []dockerobs.ContainerSummary) (map[string]int, map[string]int, map[string]int) {
	images := map[string]int{}
	volumes := map[string]int{}
	networks := map[string]int{}
	for _, item := range containers {
		images[item.ImageID]++
		for _, m := range item.Mounts {
			if m.Type == "volume" && m.VolumeName != "" {
				volumes[m.VolumeName]++
			}
		}
		for _, n := range item.Networks {
			networks[n]++
		}
	}
	return images, volumes, networks
}

// firstContainerName returns one current name of a container summary.
func firstContainerName(s dockerobs.ContainerSummary) string {
	if len(s.Names) > 0 {
		return s.Names[0]
	}
	return ""
}

// pagedProjects applies response-budget-aware pagination to sorted typed
// rows. The default page derives from the response ceiling so a full
// observation never discards the whole result at the backend output bound;
// explicit limits above the budget clamp with a next page instead.
func (c *Collector) pagedProjects[T any](r *contract.Result, a contract.PageArgs, items []T) {
	budget := c.Config.Limits.ResponseBytes / 2048
	if budget < 1 {
		budget = 1
	}
	limit := a.Limit
	if limit == 0 {
		limit = c.Config.Limits.PageSize
	}
	if limit > budget {
		limit = budget
	}
	if limit < 1 || limit > c.Config.Limits.PageSize || a.Offset < 0 {
		r.Error = true
		r.Issue("invalid_bounds", "", "page bounds exceed ceiling")
		return
	}
	start := min(a.Offset, len(items))
	end := min(start+limit, len(items))
	page := dockerobs.Page[T]{SnapshotConsistent: false}
	if len(items) == 0 {
		page.Items = items
	} else {
		page.Items = items[start:end]
	}
	r.Data = &page
	if end < len(items) {
		r.NextOffset = &end
	}
}

// dockerAuthorized enforces the class-level grant before any observer
// access, mirroring the audit collector's policy re-check. Collection forms
// evaluate allow and deny directly; item classes require an active allow
// and no class-wide denial.
func (c *Collector) dockerAuthorized(tool string) bool {
	kind := dockerToolKinds[tool]
	switch kind {
	case "daemon", "containers", "images", "volumes", "networks", "disk_usage":
		return c.Policy.Allowed("docker", kind, false)
	default:
		granted := false
		for _, k := range c.Policy.ActiveKinds("docker") {
			if k == kind {
				granted = true
				break
			}
		}
		return granted && !c.Policy.DockerKindDenied(kind)
	}
}

// docker dispatches the registered Docker tools. Rejections happen before
// observer access; every result derives from live observations only.
func (c *Collector) docker(ctx context.Context, tool string, args any) contract.Result {
	if !c.dockerEnabled() {
		return contract.Failure("docker_disabled")
	}
	if !c.dockerAuthorized(tool) {
		r := contract.Failure("policy_denied")
		r.Issue("policy_denied", "docker", fmt.Sprintf("%s is denied by policy or lacks an active grant", tool))
		return r
	}
	r := c.result()
	r.Source = "docker-observer"
	// Seed the typed payload before dispatch so policy, selector, and
	// observation failures marshal schema-valid zero payloads instead of an
	// empty object the published output schema would reject.
	switch tool {
	case "get_docker_info":
		r.Data = &dockerobs.EngineInfoPayload{}
		return c.dockerInfo(ctx, &r)
	case "list_docker_containers":
		if a, ok := args.(contract.PageArgs); ok {
			r.Data = &dockerobs.Page[dockerobs.ContainerPayload]{Items: []dockerobs.ContainerPayload{}}
			return c.dockerContainers(ctx, &r, a)
		}
	case "get_docker_container":
		if a, ok := args.(contract.ContainerArgs); ok {
			r.Data = &dockerobs.ContainerDetailPayload{ContainerPayload: dockerobs.ContainerPayload{Names: []string{}}}
			return c.dockerContainer(ctx, &r, a)
		}
	case "get_docker_container_stats":
		if a, ok := args.(contract.ContainerArgs); ok {
			r.Data = &dockerobs.ContainerStatsPayload{}
			return c.dockerStats(ctx, &r, a)
		}
	case "list_docker_images":
		if a, ok := args.(contract.PageArgs); ok {
			r.Data = &dockerobs.Page[dockerobs.ImagePayload]{Items: []dockerobs.ImagePayload{}}
			return c.dockerImages(ctx, &r, a)
		}
	case "list_docker_volumes":
		if a, ok := args.(contract.PageArgs); ok {
			r.Data = &dockerobs.Page[dockerobs.VolumePayload]{Items: []dockerobs.VolumePayload{}}
			return c.dockerVolumes(ctx, &r, a)
		}
	case "list_docker_networks":
		if a, ok := args.(contract.PageArgs); ok {
			r.Data = &dockerobs.Page[dockerobs.NetworkPayload]{Items: []dockerobs.NetworkPayload{}}
			return c.dockerNetworks(ctx, &r, a)
		}
	case "get_docker_disk_usage":
		r.Data = &dockerobs.DiskUsagePayload{
			Images:     []dockerobs.ImagePayload{},
			Containers: []dockerobs.ContainerRefPayload{},
			Volumes:    []dockerobs.VolumePayload{},
		}
		return c.dockerDiskUsage(ctx, &r)
	case "query_docker_logs":
		if a, ok := args.(contract.DockerLogsArgs); ok {
			r.Data = &dockerobs.DockerLogPage{LogPagePayload: dockerobs.LogPagePayload{Entries: []dockerobs.LogRecord{}}}
			return c.dockerLogs(ctx, &r, a)
		}
	default:
		return contract.Failure("unsupported_operation")
	}
	return contract.Failure("invalid_arguments")
}

// dockerInfo projects the engine identity and capability state. Unsupported
// modes stay explicitly unavailable instead of claiming system-wide scope.
func (c *Collector) dockerInfo(ctx context.Context, r *contract.Result) contract.Result {
	response, e := c.observe(ctx, dockerobs.Request{Version: dockerobs.ProtocolVersion, Operation: dockerobs.OpEngineInfo})
	if e != nil {
		return c.dockerUnavailable(ctx, e)
	}
	if response.Failed {
		r.Error = true
		r.Issue("docker_unavailable", r.Source, response.Reason)
		return *r
	}
	engine := response.Engine
	if engine == nil {
		r.Error = true
		r.Issue("docker_unavailable", r.Source, "observer returned no engine projection")
		return *r
	}
	payload := dockerobs.ToEngineInfo(engine)
	if len(engine.UnsupportedReasons) > 0 {
		r.Error = true
		r.Issue("unsupported_engine", r.Source, "engine mode unsupported: "+strings.Join(engine.UnsupportedReasons, ", "))
		payload.Available = false
		r.Data = &payload
		return *r
	}
	payload.Available = true
	r.Data = &payload
	return *r
}

func (c *Collector) dockerContainers(ctx context.Context, r *contract.Result, a contract.PageArgs) contract.Result {
	response, e := c.observe(ctx, dockerobs.Request{Version: dockerobs.ProtocolVersion, Operation: dockerobs.OpContainerList})
	if e != nil {
		return c.dockerUnavailable(ctx, e)
	}
	if response.Failed {
		r.Error = true
		r.Issue("docker_unavailable", r.Source, response.Reason)
		return *r
	}
	if response.Truncated {
		r.Truncated = true
		r.Issue("inventory_ceiling", r.Source, "container inventory exceeded the observation ceiling")
	}
	filtered := 0
	kept := make([]dockerobs.ContainerSummary, 0, len(response.Containers))
	for _, item := range response.Containers {
		if !c.Policy.DockerListDecision("container", firstContainerName(item), item.ID, "containers") {
			filtered++
			continue
		}
		kept = append(kept, item)
	}
	dockerobs.SortContainerIDs(kept)
	items := make([]dockerobs.ContainerPayload, 0, len(kept))
	for i := range kept {
		items = append(items, dockerobs.ToContainer(kept[i]))
	}
	c.pagedProjects(r, a, items)
	if filtered > 0 {
		r.Issue("policy_filtered", "docker", fmt.Sprintf("%d containers omitted by explicit denial", filtered))
	}
	return *r
}

func (c *Collector) dockerContainer(ctx context.Context, r *contract.Result, a contract.ContainerArgs) contract.Result {
	if a.Container == "" {
		r.Error = true
		r.Issue("invalid_selector", "", "one container selector is required")
		return *r
	}
	id, _, e := c.resolveContainerLive(ctx, a.Container)
	if e != nil {
		selectorFailure(r, a.Container, e)
		return *r
	}
	if !c.Policy.DockerDecision("container", a.Container, id) {
		deniedResult(r, "container", a.Container)
		return *r
	}
	response, e := c.observe(ctx, dockerobs.Request{Version: dockerobs.ProtocolVersion, Operation: dockerobs.OpContainerDetail, Selector: id})
	if e != nil {
		return c.dockerUnavailable(ctx, e)
	}
	if response.Failed {
		observationFailure(r, a.Container, response)
		return *r
	}
	if response.Detail == nil || response.Detail.ID != id {
		r.Error = true
		r.Issue("identity_changed", a.Container, "resolved identity did not match the observed container")
		return *r
	}
	if e := c.recheckContainerIdentity(ctx, a.Container, id); e != nil {
		r.Error = true
		r.Issue("identity_changed", a.Container, e.Error())
		return *r
	}
	detail := dockerobs.ToContainerDetail(response.Detail)
	r.Data = &detail
	return *r
}

func (c *Collector) dockerStats(ctx context.Context, r *contract.Result, a contract.ContainerArgs) contract.Result {
	if a.Container == "" {
		r.Error = true
		r.Issue("invalid_selector", "", "one container selector is required")
		return *r
	}
	id, summary, e := c.resolveContainerLive(ctx, a.Container)
	if e != nil {
		selectorFailure(r, a.Container, e)
		return *r
	}
	if !c.Policy.DockerDecision("stats", a.Container, id) {
		deniedResult(r, "stats", a.Container)
		return *r
	}
	response, e := c.observe(ctx, dockerobs.Request{Version: dockerobs.ProtocolVersion, Operation: dockerobs.OpContainerStats, Selector: id})
	if e != nil {
		return c.dockerUnavailable(ctx, e)
	}
	if response.Failed {
		observationFailure(r, a.Container, response)
		return *r
	}
	if response.Stats == nil {
		r.Error = true
		r.Issue("docker_unavailable", a.Container, "observer returned no stats projection")
		return *r
	}
	if e := c.recheckContainerIdentity(ctx, a.Container, id); e != nil {
		r.Error = true
		r.Issue("identity_changed", a.Container, e.Error())
		return *r
	}
	stats := dockerobs.ToContainerStats(response.Stats)
	r.Data = &stats
	if summary.State != "running" {
		r.Issue("container_not_running", a.Container, fmt.Sprintf("observed state %q supplies no live sample", summary.State))
	}
	return *r
}

func (c *Collector) dockerImages(ctx context.Context, r *contract.Result, a contract.PageArgs) contract.Result {
	response, containers, stable, e := c.correlatedInventory(ctx, dockerobs.OpImageList)
	if e != nil {
		return c.dockerUnavailable(ctx, e)
	}
	if response.Truncated {
		r.Truncated = true
		r.Issue("inventory_ceiling", r.Source, "image inventory exceeded the observation ceiling")
	}
	refs, _, _ := containerReferences(containers)
	filtered := 0
	kept := make([]dockerobs.ImageSummary, 0, len(response.Images))
	for _, item := range response.Images {
		// Every tag is a practical identity: denial through any tag form
		// excludes the image from the inventory.
		if imageDenied(c.Policy, item) {
			filtered++
			continue
		}
		count := refs[item.ID]
		item.ContainerRefs = &count
		item.CurrentlyUnused = stable && count == 0
		kept = append(kept, item)
	}
	if !stable && len(response.Images) > 0 {
		r.Issue("non_atomic_observation", r.Source, "container references changed during collection; unused state is uncertain")
	}
	dockerobs.SortImageIDs(kept)
	items := make([]dockerobs.ImagePayload, 0, len(kept))
	for i := range kept {
		items = append(items, dockerobs.ToImage(kept[i]))
	}
	c.pagedProjects(r, a, items)
	if filtered > 0 {
		r.Issue("policy_filtered", "docker", fmt.Sprintf("%d images omitted by explicit denial", filtered))
	}
	return *r
}

func (c *Collector) dockerVolumes(ctx context.Context, r *contract.Result, a contract.PageArgs) contract.Result {
	response, containers, stable, e := c.correlatedInventory(ctx, dockerobs.OpVolumeList)
	if e != nil {
		return c.dockerUnavailable(ctx, e)
	}
	if response.Truncated {
		r.Truncated = true
		r.Issue("inventory_ceiling", r.Source, "volume inventory exceeded the observation ceiling")
	}
	_, refs, _ := containerReferences(containers)
	filtered := 0
	kept := make([]dockerobs.VolumeSummary, 0, len(response.Volumes))
	for _, item := range response.Volumes {
		if !c.Policy.DockerListDecision("volume", item.Name, item.Name, "volumes") {
			filtered++
			continue
		}
		if item.RefCount == nil {
			count := refs[item.Name]
			item.RefCount = &count
		}
		// A daemon-supplied reference count is current evidence; a derived
		// count replaces it only when the daemon supplied none.
		item.CurrentlyUnused = stable && item.RefCount != nil && *item.RefCount == 0
		kept = append(kept, item)
	}
	if !stable && len(response.Volumes) > 0 {
		r.Issue("non_atomic_observation", r.Source, "container references changed during collection; unused state is uncertain")
	}
	dockerobs.SortVolumeNames(kept)
	items := make([]dockerobs.VolumePayload, 0, len(kept))
	for i := range kept {
		items = append(items, dockerobs.ToVolume(kept[i]))
	}
	c.pagedProjects(r, a, items)
	if filtered > 0 {
		r.Issue("policy_filtered", "docker", fmt.Sprintf("%d volumes omitted by explicit denial", filtered))
	}
	return *r
}

func (c *Collector) dockerNetworks(ctx context.Context, r *contract.Result, a contract.PageArgs) contract.Result {
	response, containers, stable, e := c.correlatedInventory(ctx, dockerobs.OpNetworkList)
	if e != nil {
		return c.dockerUnavailable(ctx, e)
	}
	if response.Truncated {
		r.Truncated = true
		r.Issue("inventory_ceiling", r.Source, "network inventory exceeded the observation ceiling")
	}
	_, _, netRefs := containerReferences(containers)
	filtered := 0
	kept := make([]dockerobs.NetworkSummary, 0, len(response.Networks))
	for _, item := range response.Networks {
		// The current name is a practical identity for deny precedence.
		if !c.Policy.DockerListDecision("network", item.Name, item.ID, "networks") {
			filtered++
			continue
		}
		count := netRefs[item.Name]
		item.ContainerRefs = &count
		kept = append(kept, item)
	}
	if !stable && len(response.Networks) > 0 {
		r.Issue("non_atomic_observation", r.Source, "container references changed during collection; unused state is uncertain")
	}
	dockerobs.SortNetworkIDs(kept)
	items := make([]dockerobs.NetworkPayload, 0, len(kept))
	for i := range kept {
		items = append(items, dockerobs.ToNetwork(kept[i]))
	}
	c.pagedProjects(r, a, items)
	if filtered > 0 {
		r.Issue("policy_filtered", "docker", fmt.Sprintf("%d networks omitted by explicit denial", filtered))
	}
	return *r
}

func (c *Collector) dockerDiskUsage(ctx context.Context, r *contract.Result) contract.Result {
	response, containers, stable, e := c.correlatedInventory(ctx, dockerobs.OpDiskUsage)
	if e != nil {
		return c.dockerUnavailable(ctx, e)
	}
	if response.Usage == nil {
		r.Error = true
		r.Issue("docker_unavailable", r.Source, "observer returned no disk usage projection")
		return *r
	}
	if !c.Policy.Allowed("docker", "disk_usage", false) {
		deniedResult(r, "disk_usage", "")
		return *r
	}
	refs, volumeRefs, _ := containerReferences(containers)
	usage := response.Usage
	reclaimable := &dockerobs.Reclaimable{ObservationStable: stable}
	var unusedImageBytes, unusedVolumeBytes int64
	var unusedImages, unusedVolumes, dangling []string
	filteredImages, filteredContainers := 0, 0
	keptImages := make([]dockerobs.ImageSummary, 0, len(usage.Images))
	for _, image := range usage.Images {
		if imageDenied(c.Policy, image) {
			filteredImages++
			continue
		}
		count := refs[image.ID]
		image.ContainerRefs = &count
		image.CurrentlyUnused = stable && count == 0
		if image.CurrentlyUnused {
			estimate := image.Size
			if image.SharedSize != nil && estimate != nil {
				unique := *image.Size - *image.SharedSize
				if unique < 0 {
					unique = 0
				}
				estimate = &unique
			}
			if estimate != nil {
				unusedImageBytes += *estimate
			}
			unusedImages = append(unusedImages, image.ID)
			if image.Dangling {
				dangling = append(dangling, image.ID)
			}
		}
		keptImages = append(keptImages, image)
	}
	usage.Images = keptImages
	keptContainers := make([]dockerobs.ContainerRef, 0, len(usage.Containers))
	for _, container := range usage.Containers {
		summary, ok := containerByID(containers, container.ID)
		name := ""
		if ok {
			name = firstContainerName(*summary)
		}
		if !c.Policy.DockerListDecision("container", name, container.ID, "containers") {
			filteredContainers++
			continue
		}
		keptContainers = append(keptContainers, container)
	}
	usage.Containers = keptContainers
	filteredVolumes := 0
	keptVolumes := make([]dockerobs.VolumeSummary, 0, len(usage.Volumes))
	for _, volume := range usage.Volumes {
		if !c.Policy.DockerListDecision("volume", "", volume.Name, "volumes") {
			filteredVolumes++
			continue
		}
		if volume.RefCount == nil {
			count := volumeRefs[volume.Name]
			volume.RefCount = &count
		}
		volume.CurrentlyUnused = stable && volume.RefCount != nil && *volume.RefCount == 0
		if volume.CurrentlyUnused {
			if volume.Size != nil {
				unusedVolumeBytes += *volume.Size
			}
			unusedVolumes = append(unusedVolumes, volume.Name)
		}
		keptVolumes = append(keptVolumes, volume)
	}
	usage.Volumes = keptVolumes
	// The daemon-supplied reference counts take precedence; mount-based
	// counts are only the correlation fallback.
	reclaimable.UnusedImages = unusedImages
	reclaimable.UnusedImageBytes = &unusedImageBytes
	reclaimable.UnusedVolumes = unusedVolumes
	reclaimable.UnusedVolumeBytes = &unusedVolumeBytes
	reclaimable.DanglingImages = dangling
	for _, item := range containers {
		switch item.State {
		case "exited", "created", "dead", "removing":
			// Stopped-container evidence follows the container policy: a
			// denied container contributes neither its ID nor its footprint.
			if !c.Policy.DockerListDecision("container", firstContainerName(item), item.ID, "containers") {
				filteredContainers++
				continue
			}
			reclaimable.StoppedContainers = append(reclaimable.StoppedContainers, item.ID)
		}
	}
	usage.Reclaimable = reclaimable
	usageData := dockerobs.ToDiskUsage(usage)
	r.Data = &usageData
	if !stable {
		r.Issue("non_atomic_observation", r.Source, "docker changed during accounting; reclaimable estimates are non-atomic")
	}
	if filteredImages > 0 || filteredVolumes > 0 || filteredContainers > 0 {
		r.Issue("policy_filtered", "docker", fmt.Sprintf("%d images, %d volumes and %d containers omitted by explicit denial", filteredImages, filteredVolumes, filteredContainers))
	}
	return *r
}

func containerByID(items []dockerobs.ContainerSummary, id string) (*dockerobs.ContainerSummary, bool) {
	for i := range items {
		if items[i].ID == id {
			return &items[i], true
		}
	}
	return nil, false
}

// dockerLogs retrieves one bounded, non-following log window for one
// policy-permitted container with explicit truncation and coverage gaps.
func (c *Collector) dockerLogs(ctx context.Context, r *contract.Result, a contract.DockerLogsArgs) contract.Result {
	if a.Container == "" {
		r.Error = true
		r.Issue("invalid_selector", "", "one container selector is required")
		return *r
	}
	l := c.Config.Limits
	n := a.Limit
	if n == 0 {
		n = l.LogEntries
	}
	if n < 1 || n > l.LogEntries {
		r.Error = true
		r.Issue("invalid_bounds", "", "container logs accept since, until, and limit within the entry ceiling")
		return *r
	}
	since, until, e := window(a.Since, a.Until, l)
	if e != nil {
		r.Error = true
		r.Issue("invalid_window", "", e.Error())
		return *r
	}
	id, summary, e := c.resolveContainerLive(ctx, a.Container)
	if e != nil {
		selectorFailure(r, a.Container, e)
		return *r
	}
	if !c.Policy.DockerDecision("logs", a.Container, id) {
		deniedResult(r, "logs", a.Container)
		return *r
	}
	logBytes := l.InspectionBytes
	if logBytes > dockerobs.MaxLogBytes {
		// The observer contract caps one log response regardless of the
		// shared inspection budget; larger configuration values clamp here.
		logBytes = dockerobs.MaxLogBytes
	}
	request := dockerobs.Request{
		Version:   dockerobs.ProtocolVersion,
		Operation: dockerobs.OpContainerLogs,
		Selector:  id,
		Logs: dockerobs.LogOptions{
			Since:    since,
			Until:    until,
			Records:  n,
			MaxBytes: logBytes,
		},
	}
	response, e := c.observe(ctx, request)
	if e != nil {
		return c.dockerUnavailable(ctx, e)
	}
	if response.Failed {
		if response.Issue == "unsupported_driver" {
			// The observer classifies unsupported log drivers structurally;
			// the gap never claims that no logs or incidents exist.
			r.Error = true
			r.Issue("driver_gap", a.Container, response.Reason)
			return *r
		}
		observationFailure(r, a.Container, response)
		return *r
	}
	if response.Logs == nil {
		r.Error = true
		r.Issue("docker_unavailable", a.Container, "observer returned no log projection")
		return *r
	}
	if e := c.recheckContainerIdentity(ctx, a.Container, id); e != nil {
		r.Error = true
		r.Issue("identity_changed", a.Container, e.Error())
		return *r
	}
	logs := response.Logs
	r.Truncated = logs.Truncated
	r.Data = &dockerobs.DockerLogPage{
		LogPagePayload: dockerobs.ToLogPage(logs),
		RequestedSince: since.Format(time.RFC3339Nano),
		RequestedUntil: until.Format(time.RFC3339Nano),
	}
	r.Source = "docker-observer:" + id
	if summary.State != "running" && summary.State != "" {
		r.Issue("container_not_running", a.Container, fmt.Sprintf("observed state %q; returned records are historical", summary.State))
	}
	r.Issue("retention_unverified", a.Container, "rotation coverage of the daemon log driver cannot be verified; the returned window may omit rotated history")
	return *r
}
