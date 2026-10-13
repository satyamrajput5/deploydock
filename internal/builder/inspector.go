package builder

import (
	"bufio"
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/satyamrajput5/deploydock/internal/models"
)

var secretKeywords = []string{
	"SECRET",
	"PASSWORD",
	"PASSWD",
	"TOKEN",
	"KEY",
	"AUTH",
	"PRIVATE",
	"DATABASE",
	"DB_",
	"CONN",
	"CREDENTIAL",
	"APIKEY",
	"API_KEY",
	"CERT",
	"SIGNING",
	"JWT",
	"ENCRYPT",
}

func isSecretKey(key string) bool {
	upper := strings.ToUpper(key)
	for _, kw := range secretKeywords {
		if strings.Contains(upper, kw) {
			return true
		}
	}
	return false
}

func FetchBranches(ctx context.Context, repoURL string) ([]string, string, error) {
	branchesCmd := exec.CommandContext(ctx, "git", "ls-remote", "--heads", repoURL)
	out, err := branchesCmd.Output()
	if err != nil {
		return nil, "", err
	}

	var branches []string
	branchSet := make(map[string]bool)
	lines := strings.Split(string(out), "\n")
	prefix := "refs/heads/"
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.Fields(line)
		if len(parts) >= 2 {
			ref := parts[1]
			if strings.HasPrefix(ref, prefix) {
				branchName := strings.TrimPrefix(ref, prefix)
				if branchName != "" && !branchSet[branchName] {
					branches = append(branches, branchName)
					branchSet[branchName] = true
				}
			}
		}
	}

	defaultBranch := "main"
	symrefCmd := exec.CommandContext(ctx, "git", "ls-remote", "--symref", repoURL, "HEAD")
	symrefOut, err := symrefCmd.Output()
	if err == nil {
		symrefLines := strings.Split(string(symrefOut), "\n")
		for _, sLine := range symrefLines {
			sLine = strings.TrimSpace(sLine)
			if strings.HasPrefix(sLine, "ref: refs/heads/") {
				fields := strings.Fields(sLine)
				if len(fields) >= 2 {
					refTarget := fields[1]
					defaultBranch = strings.TrimPrefix(refTarget, prefix)
					break
				}
			}
		}
	}

	if !branchSet[defaultBranch] {
		if branchSet["main"] {
			defaultBranch = "main"
		} else if branchSet["master"] {
			defaultBranch = "master"
		} else if len(branches) > 0 {
			defaultBranch = branches[0]
		}
	}

	return branches, defaultBranch, nil
}

func parseEnvFile(filePath string) []models.EnvVarItem {
	file, err := os.Open(filePath)
	if err != nil {
		return nil
	}
	defer file.Close()

	var items []models.EnvVarItem
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if strings.HasPrefix(line, "export ") {
			line = strings.TrimSpace(strings.TrimPrefix(line, "export "))
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) == 2 {
			key := strings.TrimSpace(parts[0])
			val := strings.TrimSpace(parts[1])
			val = strings.Trim(val, "\"'")
			if key != "" {
				items = append(items, models.EnvVarItem{
					Key:      key,
					Value:    val,
					IsSecret: isSecretKey(key),
				})
			}
		}
	}
	return items
}

func flattenJSON(prefix string, val interface{}, items *[]models.EnvVarItem) {
	switch v := val.(type) {
	case map[string]interface{}:
		for k, nestedVal := range v {
			nextPrefix := k
			if prefix != "" {
				nextPrefix = prefix + "__" + k
			}
			flattenJSON(nextPrefix, nestedVal, items)
		}
	case string:
		if prefix != "" {
			*items = append(*items, models.EnvVarItem{
				Key:      prefix,
				Value:    v,
				IsSecret: isSecretKey(prefix),
			})
		}
	case float64:
		if prefix != "" {
			*items = append(*items, models.EnvVarItem{
				Key:      prefix,
				Value:    strings.TrimSuffix(strings.TrimRight(strings.TrimRight(string([]byte(strings.TrimSpace(strings.ReplaceAll(string([]byte(string([]byte("")))), "", "")))), ""), ""), ""),
				IsSecret: isSecretKey(prefix),
			})
		}
	}
}

func parseAppSettingsFile(filePath string) []models.EnvVarItem {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil
	}

	var root map[string]interface{}
	if err := json.Unmarshal(data, &root); err != nil {
		return nil
	}

	var items []models.EnvVarItem
	for k, v := range root {
		flattenJSON(k, v, &items)
	}
	return items
}

func parseDockerfileEnv(filePath string) []models.EnvVarItem {
	file, err := os.Open(filePath)
	if err != nil {
		return nil
	}
	defer file.Close()

	var items []models.EnvVarItem
	scanner := bufio.NewScanner(file)
	envRegex := regexp.MustCompile(`^ENV\s+(.+)`)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		match := envRegex.FindStringSubmatch(line)
		if len(match) == 2 {
			rest := strings.TrimSpace(match[1])
			if strings.Contains(rest, "=") {
				parts := strings.SplitN(rest, "=", 2)
				key := strings.TrimSpace(parts[0])
				val := strings.Trim(strings.TrimSpace(parts[1]), "\"'")
				if key != "" {
					items = append(items, models.EnvVarItem{
						Key:      key,
						Value:    val,
						IsSecret: isSecretKey(key),
					})
				}
			} else {
				fields := strings.Fields(rest)
				if len(fields) >= 2 {
					key := fields[0]
					val := strings.Trim(fields[1], "\"'")
					items = append(items, models.EnvVarItem{
						Key:      key,
						Value:    val,
						IsSecret: isSecretKey(key),
					})
				}
			}
		}
	}
	return items
}

func ScanEnvVars(repoDir string) []models.EnvVarItem {
	var collected []models.EnvVarItem
	seen := make(map[string]bool)

	candidateFiles := []string{
		".env.example",
		".env.sample",
		".env.template",
		".env.dist",
		".env.local.example",
		".env",
		"appsettings.json",
		"appsettings.Development.json",
		"config.example.json",
		"Dockerfile",
		"Dockerfile.prod",
		"Dockerfile.deploydock",
	}

	for _, cand := range candidateFiles {
		fullPath := filepath.Join(repoDir, cand)
		if _, err := os.Stat(fullPath); err == nil {
			var fileItems []models.EnvVarItem
			if strings.HasSuffix(cand, ".json") {
				fileItems = parseAppSettingsFile(fullPath)
			} else if strings.Contains(cand, "Dockerfile") {
				fileItems = parseDockerfileEnv(fullPath)
			} else {
				fileItems = parseEnvFile(fullPath)
			}

			for _, it := range fileItems {
				if !seen[it.Key] {
					seen[it.Key] = true
					collected = append(collected, it)
				}
			}
		}
	}

	_ = filepath.Walk(repoDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info == nil {
			return nil
		}
		if info.IsDir() {
			name := info.Name()
			if name == ".git" || name == "node_modules" || name == "vendor" || name == "bin" || name == "obj" {
				return filepath.SkipDir
			}
			return nil
		}
		base := info.Name()
		if base == ".env.example" || base == ".env.sample" || base == "appsettings.json" {
			var subItems []models.EnvVarItem
			if strings.HasSuffix(base, ".json") {
				subItems = parseAppSettingsFile(path)
			} else {
				subItems = parseEnvFile(path)
			}
			for _, it := range subItems {
				if !seen[it.Key] {
					seen[it.Key] = true
					collected = append(collected, it)
				}
			}
		}
		return nil
	})

	return collected
}

func InspectRepo(ctx context.Context, repoURL string) (*models.InspectRepoResponse, error) {
	branches, defaultBranch, err := FetchBranches(ctx, repoURL)
	if err != nil {
		return nil, err
	}

	tempDir, err := os.MkdirTemp("", "deploydock-inspect-*")
	if err != nil {
		return &models.InspectRepoResponse{
			RepoURL:         repoURL,
			DefaultBranch:   defaultBranch,
			Branches:        branches,
			DetectedEnvVars: []models.EnvVarItem{},
		}, nil
	}
	defer os.RemoveAll(tempDir)

	cloneCtx, cloneCancel := context.WithTimeout(ctx, 15*time.Second)
	defer cloneCancel()

	cloneCmd := exec.CommandContext(cloneCtx, "git", "clone", "--depth", "1", "--branch", defaultBranch, repoURL, tempDir)
	if err := cloneCmd.Run(); err != nil {
		fallbackCmd := exec.CommandContext(cloneCtx, "git", "clone", "--depth", "1", repoURL, tempDir)
		_ = fallbackCmd.Run()
	}

	detected := ScanEnvVars(tempDir)
	if detected == nil {
		detected = []models.EnvVarItem{}
	}

	return &models.InspectRepoResponse{
		RepoURL:         repoURL,
		DefaultBranch:   defaultBranch,
		Branches:        branches,
		DetectedEnvVars: detected,
	}, nil
}
