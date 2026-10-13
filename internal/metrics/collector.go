package metrics

import (
	"context"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
)

type ContainerMetric struct {
	ContainerID string    `json:"container_id"`
	DeploymentID int      `json:"deployment_id"`
	BuildID     int       `json:"build_id"`
	Name        string    `json:"name"`
	CPUPct      float64   `json:"cpu_pct"`
	MemMB       float64   `json:"mem_mb"`
	MemPerc     float64   `json:"mem_perc"`
	MemUsageRaw string    `json:"mem_usage_raw"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type OverviewMetric struct {
	AvgCPUPct    float64                   `json:"avg_cpu_pct"`
	AvgMemMB     float64                   `json:"avg_mem_mb"`
	AvgMemPerc   float64                   `json:"avg_mem_perc"`
	ActiveCount  int                       `json:"active_count"`
	Containers   map[string]ContainerMetric `json:"containers"`
	ByDeployment map[int]ContainerMetric   `json:"by_deployment"`
	UpdatedAt    time.Time                 `json:"updated_at"`
}

type Collector struct {
	mu       sync.RWMutex
	overview OverviewMetric
	nameRe   *regexp.Regexp
}

func NewCollector() *Collector {
	return &Collector{
		nameRe: regexp.MustCompile(`deploydock-app-(\d+)-(\d+)`),
		overview: OverviewMetric{
			Containers:   make(map[string]ContainerMetric),
			ByDeployment: make(map[int]ContainerMetric),
		},
	}
}

func (c *Collector) Start(ctx context.Context) {
	go func() {
		c.poll()
		ticker := time.NewTicker(3 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				c.poll()
			}
		}
	}()
}

func (c *Collector) GetOverview() OverviewMetric {
	c.mu.RLock()
	defer c.mu.RUnlock()

	containersCopy := make(map[string]ContainerMetric, len(c.overview.Containers))
	for k, v := range c.overview.Containers {
		containersCopy[k] = v
	}

	byDepCopy := make(map[int]ContainerMetric, len(c.overview.ByDeployment))
	for k, v := range c.overview.ByDeployment {
		byDepCopy[k] = v
	}

	return OverviewMetric{
		AvgCPUPct:    c.overview.AvgCPUPct,
		AvgMemMB:     c.overview.AvgMemMB,
		AvgMemPerc:   c.overview.AvgMemPerc,
		ActiveCount:  c.overview.ActiveCount,
		Containers:   containersCopy,
		ByDeployment: byDepCopy,
		UpdatedAt:    c.overview.UpdatedAt,
	}
}

func (c *Collector) GetDeploymentMetric(deploymentID int) (ContainerMetric, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	metric, ok := c.overview.ByDeployment[deploymentID]
	return metric, ok
}

func (c *Collector) poll() {
	cmd := exec.Command("docker", "stats", "--no-stream", "--format", "{{.ID}}|{{.CPUPerc}}|{{.MemUsage}}|{{.MemPerc}}|{{.Name}}")
	out, err := cmd.Output()
	if err != nil {
		return
	}

	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	containers := make(map[string]ContainerMetric)
	byDep := make(map[int]ContainerMetric)

	var totalCPU float64
	var totalMemMB float64
	var totalMemPerc float64
	var count int

	now := time.Now()

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.Split(line, "|")
		if len(parts) < 5 {
			continue
		}

		cid := strings.TrimSpace(parts[0])
		cpuStr := strings.TrimSuffix(strings.TrimSpace(parts[1]), "%")
		cpu, _ := strconv.ParseFloat(cpuStr, 64)

		memMB := parseMemMB(parts[2])

		memPercStr := strings.TrimSuffix(strings.TrimSpace(parts[3]), "%")
		memPerc, _ := strconv.ParseFloat(memPercStr, 64)

		name := strings.TrimSpace(parts[4])

		depID := 0
		buildID := 0
		if sub := c.nameRe.FindStringSubmatch(name); len(sub) >= 3 {
			depID, _ = strconv.Atoi(sub[1])
			buildID, _ = strconv.Atoi(sub[2])
		}

		metric := ContainerMetric{
			ContainerID:  cid,
			DeploymentID: depID,
			BuildID:      buildID,
			Name:         name,
			CPUPct:       cpu,
			MemMB:        memMB,
			MemPerc:      memPerc,
			MemUsageRaw:  strings.TrimSpace(parts[2]),
			UpdatedAt:    now,
		}

		containers[cid] = metric
		if depID > 0 {
			byDep[depID] = metric
		}

		totalCPU += cpu
		totalMemMB += memMB
		totalMemPerc += memPerc
		count++
	}

	var avgCPU float64
	var avgMemMB float64
	var avgMemPerc float64

	if count > 0 {
		avgCPU = totalCPU / float64(count)
		avgMemMB = totalMemMB / float64(count)
		avgMemPerc = totalMemPerc / float64(count)
	}

	c.mu.Lock()
	c.overview = OverviewMetric{
		AvgCPUPct:    avgCPU,
		AvgMemMB:     avgMemMB,
		AvgMemPerc:   avgMemPerc,
		ActiveCount:  count,
		Containers:   containers,
		ByDeployment: byDep,
		UpdatedAt:    now,
	}
	c.mu.Unlock()
}

func parseMemMB(memUsage string) float64 {
	slashIdx := strings.Index(memUsage, "/")
	used := memUsage
	if slashIdx != -1 {
		used = strings.TrimSpace(memUsage[:slashIdx])
	}
	used = strings.TrimSpace(used)
	if used == "" {
		return 0
	}

	unitIdx := -1
	for i, r := range used {
		if (r < '0' || r > '9') && r != '.' {
			unitIdx = i
			break
		}
	}
	if unitIdx == -1 {
		v, _ := strconv.ParseFloat(used, 64)
		return v
	}
	val, err := strconv.ParseFloat(used[:unitIdx], 64)
	if err != nil {
		return 0
	}
	unit := strings.ToLower(strings.TrimSpace(used[unitIdx:]))
	switch unit {
	case "gib", "gb":
		return val * 1024
	case "mib", "mb":
		return val
	case "kib", "kb":
		return val / 1024
	case "b":
		return val / (1024 * 1024)
	default:
		return val
	}
}
