package builder

import (
	"context"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/jmoiron/sqlx"
)

func Clone(ctx context.Context, repoURL, branch, commitSHA, destDir string, out io.Writer) error {
	if err := os.RemoveAll(destDir); err != nil {
		return fmt.Errorf("failed cleaning destination dir: %w", err)
	}

	if err := os.MkdirAll(destDir, 0755); err != nil {
		return fmt.Errorf("failed creating destination dir: %w", err)
	}

	cmd := exec.CommandContext(ctx, "git", "clone", "--depth", "1", "--branch", branch, repoURL, destDir)
	if out != nil {
		cmd.Stdout = out
		cmd.Stderr = out
	}
	if err := cmd.Run(); err != nil {
		fallbackCmd := exec.CommandContext(ctx, "git", "clone", repoURL, destDir)
		if out != nil {
			fallbackCmd.Stdout = out
			fallbackCmd.Stderr = out
		}
		if fErr := fallbackCmd.Run(); fErr != nil {
			return fmt.Errorf("git clone failed: %w", fErr)
		}
	}

	if commitSHA != "" {
		checkoutCmd := exec.CommandContext(ctx, "git", "-C", destDir, "checkout", commitSHA)
		if out != nil {
			checkoutCmd.Stdout = out
			checkoutCmd.Stderr = out
		}
		_ = checkoutCmd.Run()
	}

	return nil
}

func Build(ctx context.Context, imageTag, contextDir, dockerfile string, out io.Writer) error {
	args := []string{"build", "-t", imageTag}
	if dockerfile != "" {
		args = append(args, "-f", dockerfile)
	}
	args = append(args, contextDir)

	cmd := exec.CommandContext(ctx, "docker", args...)
	if out != nil {
		cmd.Stdout = out
		cmd.Stderr = out
	}
	return cmd.Run()
}

func Run(ctx context.Context, deploymentID, buildID int, imageTag string, hostPort int, env map[string]string) (string, error) {
	containerName := fmt.Sprintf("deploydock-app-%d-%d", deploymentID, buildID)

	args := []string{
		"run", "-d",
		"--name", containerName,
		"-p", fmt.Sprintf("%d:8080", hostPort),
		"--cpus", "0.5",
		"--memory", "256m",
		"--pids-limit", "128",
		"--restart", "on-failure:5",
		"--label", fmt.Sprintf("deploydock.deployment_id=%d", deploymentID),
		"--label", fmt.Sprintf("deploydock.build_id=%d", buildID),
	}

	for k, v := range env {
		args = append(args, "-e", fmt.Sprintf("%s=%s", k, v))
	}

	args = append(args, imageTag)

	cmd := exec.CommandContext(ctx, "docker", args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("docker run failed: %s: %w", strings.TrimSpace(string(output)), err)
	}

	containerID := strings.TrimSpace(string(output))
	if len(containerID) > 12 {
		containerID = containerID[:12]
	}

	return containerID, nil
}

func Stop(ctx context.Context, containerID string) error {
	cmd := exec.CommandContext(ctx, "docker", "rm", "-f", containerID)
	return cmd.Run()
}

type ContainerInspect struct {
	Status     string
	Running    bool
	Restarting bool
	ExitCode   int
}

func Inspect(ctx context.Context, containerID string) (*ContainerInspect, error) {
	cmd := exec.CommandContext(ctx, "docker", "inspect", "-f", "{{.State.Status}}|{{.State.Running}}|{{.State.Restarting}}|{{.State.ExitCode}}", containerID)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("docker inspect failed: %w", err)
	}
	parts := strings.Split(strings.TrimSpace(string(out)), "|")
	if len(parts) < 4 {
		return nil, fmt.Errorf("unexpected inspect output: %s", string(out))
	}
	status := parts[0]
	running := parts[1] == "true"
	restarting := parts[2] == "true"
	var exitCode int
	_, _ = fmt.Sscanf(parts[3], "%d", &exitCode)

	return &ContainerInspect{
		Status:     status,
		Running:    running,
		Restarting: restarting,
		ExitCode:   exitCode,
	}, nil
}

func GetLogs(ctx context.Context, containerID string, tailLines int) string {
	cmd := exec.CommandContext(ctx, "docker", "logs", "--tail", fmt.Sprintf("%d", tailLines), containerID)
	out, _ := cmd.CombinedOutput()
	return strings.TrimSpace(string(out))
}

func isPortAvailable(port int) bool {
	conn, err := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", port), 150*time.Millisecond)
	if err == nil {
		_ = conn.Close()
		return false
	}

	ln, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		return false
	}
	_ = ln.Close()

	return true
}

func AllocatePort(db *sqlx.DB) (int, error) {
	usedPorts := make(map[int]bool)

	var ports []int
	query := `SELECT port FROM public.deployment_builds WHERE port IS NOT NULL AND status IN ('running', 'starting', 'building', 'pending')`
	_ = db.Select(&ports, query)
	for _, p := range ports {
		usedPorts[p] = true
	}

	var depPorts []int
	_ = db.Select(&depPorts, `SELECT port FROM public.deployments WHERE port IS NOT NULL AND status IN ('running', 'starting', 'building', 'pending')`)
	for _, p := range depPorts {
		usedPorts[p] = true
	}

	dockerCmd := exec.Command("docker", "ps", "--format", "{{.Ports}}")
	if dockerOut, err := dockerCmd.Output(); err == nil {
		re := regexp.MustCompile(`:(\d+)->`)
		matches := re.FindAllStringSubmatch(string(dockerOut), -1)
		for _, m := range matches {
			if len(m) > 1 {
				if p, pErr := strconv.Atoi(m[1]); pErr == nil {
					usedPorts[p] = true
				}
			}
		}
	}

	for port := 4000; port <= 4999; port++ {
		if !usedPorts[port] && isPortAvailable(port) {
			return port, nil
		}
	}

	return 0, fmt.Errorf("all ports in range 4000-4999 are currently allocated or occupied on the host")
}

type LiveContainerStatus struct {
	ID     string
	State  string
	Status string
	Name   string
}

func GetAllContainerStates(ctx context.Context) (map[string]LiveContainerStatus, error) {
	cmd := exec.CommandContext(ctx, "docker", "ps", "-a", "--format", "{{.ID}}|{{.State}}|{{.Status}}|{{.Names}}")
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}
	result := make(map[string]LiveContainerStatus)
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.Split(line, "|")
		if len(parts) >= 3 {
			id := parts[0]
			state := parts[1]
			status := parts[2]
			name := ""
			if len(parts) >= 4 {
				name = parts[3]
			}
			result[id] = LiveContainerStatus{
				ID:     id,
				State:  state,
				Status: status,
				Name:   name,
			}
		}
	}
	return result, nil
}
