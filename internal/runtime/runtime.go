package runtime

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/kristianvld/dtask/internal/config"
)

type Prepared struct {
	ComposeDir string
	AutoTZ     *time.Location
}

type Env interface {
	EffectiveUID() int
	Stat(path string) (os.FileInfo, error)
	ReadFile(path string) ([]byte, error)
}

type RealEnv struct{}

func (RealEnv) EffectiveUID() int {
	return os.Geteuid()
}

func (RealEnv) Stat(path string) (os.FileInfo, error) {
	return os.Stat(path)
}

func (RealEnv) ReadFile(path string) ([]byte, error) {
	return os.ReadFile(path)
}

func Prepare(cfg *config.Config, env Env) (Prepared, error) {
	if env == nil {
		env = RealEnv{}
	}

	needHost := false
	needCompose := false
	for _, task := range cfg.Tasks {
		if task.Run == config.RunHost || task.Run == config.RunCompose {
			needHost = true
		}
		if task.Run == config.RunCompose {
			needCompose = true
		}
	}

	if needHost {
		if env.EffectiveUID() != 0 {
			return Prepared{}, fmt.Errorf("run=host and run=compose require root for chroot")
		}
		st, err := env.Stat("/host")
		if err != nil || !st.IsDir() {
			return Prepared{}, fmt.Errorf("run=host and run=compose require /host mount")
		}
		for _, task := range cfg.Tasks {
			if task.Run != config.RunHost && task.Run != config.RunCompose {
				continue
			}
			shellPath := task.ShellArgv[0]
			if !strings.HasPrefix(shellPath, "/") {
				return Prepared{}, fmt.Errorf("task %q shell must be an absolute path in host/compose mode", task.Name)
			}
			if _, err := env.Stat(filepath.Join("/host", shellPath)); err != nil {
				return Prepared{}, fmt.Errorf("task %q shell %q not found on host", task.Name, shellPath)
			}
		}
	}

	autoTZ := time.Local
	if needHost {
		if tzData, err := env.ReadFile("/host/etc/timezone"); err == nil {
			if loc, loadErr := time.LoadLocation(strings.TrimSpace(string(tzData))); loadErr == nil {
				autoTZ = loc
			}
		}
	}

	prepared := Prepared{AutoTZ: autoTZ}
	if needCompose {
		composeDir, err := DetectComposeWorkingDir(env)
		if err != nil {
			return Prepared{}, err
		}
		prepared.ComposeDir = composeDir
	}

	return prepared, nil
}

func ResolveLocation(task config.Task, prepared Prepared) *time.Location {
	if task.Location != nil {
		return task.Location
	}
	if prepared.AutoTZ != nil {
		return prepared.AutoTZ
	}
	return time.Local
}

var containerIDRE = regexp.MustCompile(`[a-f0-9]{64}`)

func DetectComposeWorkingDir(env Env) (string, error) {
	if env == nil {
		env = RealEnv{}
	}

	cgroupRaw, err := env.ReadFile("/proc/self/cgroup")
	if err != nil {
		return "", fmt.Errorf("unable to read /proc/self/cgroup: %w", err)
	}
	containerID, err := ParseContainerID(string(cgroupRaw))
	if err != nil {
		return "", err
	}

	cfgPath := filepath.Join("/host/var/lib/docker/containers", containerID, "config.v2.json")
	raw, err := env.ReadFile(cfgPath)
	if err != nil {
		return "", fmt.Errorf("unable to read container metadata %q: %w", cfgPath, err)
	}

	var parsed struct {
		Config struct {
			Labels map[string]string `json:"Labels"`
		} `json:"Config"`
	}
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return "", fmt.Errorf("invalid docker metadata: %w", err)
	}

	wd := strings.TrimSpace(parsed.Config.Labels["com.docker.compose.project.working_dir"])
	if wd == "" {
		return "", fmt.Errorf("missing docker label com.docker.compose.project.working_dir")
	}
	if !filepath.IsAbs(wd) {
		return "", fmt.Errorf("compose working dir from label is not absolute: %q", wd)
	}
	return filepath.Clean(wd), nil
}

func ParseContainerID(cgroup string) (string, error) {
	matches := containerIDRE.FindAllString(cgroup, -1)
	if len(matches) > 0 {
		return matches[len(matches)-1], nil
	}

	for _, line := range strings.Split(cgroup, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.Split(line, "/")
		last := parts[len(parts)-1]
		if len(last) >= 12 && isHex(last) {
			if len(last) > 64 {
				last = last[len(last)-64:]
			}
			return last, nil
		}
	}

	return "", fmt.Errorf("unable to detect container id from /proc/self/cgroup")
}

func isHex(v string) bool {
	for _, r := range v {
		if (r < '0' || r > '9') && (r < 'a' || r > 'f') {
			return false
		}
	}
	return true
}
