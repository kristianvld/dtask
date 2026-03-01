package runtime

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/kristianvld/dtask/internal/config"
)

type fakeEnv struct {
	uid   int
	files map[string][]byte
	dirs  map[string]bool
}

func (f fakeEnv) EffectiveUID() int { return f.uid }

func (f fakeEnv) Stat(path string) (os.FileInfo, error) {
	if f.dirs[path] {
		return fakeInfo{name: filepath.Base(path), dir: true}, nil
	}
	if _, ok := f.files[path]; ok {
		return fakeInfo{name: filepath.Base(path), dir: false}, nil
	}
	return nil, os.ErrNotExist
}

func (f fakeEnv) ReadFile(path string) ([]byte, error) {
	v, ok := f.files[path]
	if !ok {
		return nil, os.ErrNotExist
	}
	return v, nil
}

type fakeInfo struct {
	name string
	dir  bool
}

func (f fakeInfo) Name() string { return f.name }
func (f fakeInfo) Size() int64  { return 0 }
func (f fakeInfo) Mode() os.FileMode {
	if f.dir {
		return os.ModeDir
	}
	return 0
}
func (f fakeInfo) ModTime() time.Time { return time.Time{} }
func (f fakeInfo) IsDir() bool        { return f.dir }
func (f fakeInfo) Sys() any           { return nil }

func TestParseContainerID(t *testing.T) {
	t.Parallel()
	id := "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	got, err := ParseContainerID("0::/docker/" + id)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if got != id {
		t.Fatalf("got=%s", got)
	}
}

func TestDetectComposeWorkingDir(t *testing.T) {
	t.Parallel()
	id := "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	metaPath := filepath.Join("/host/var/lib/docker/containers", id, "config.v2.json")
	env := fakeEnv{
		uid: 0,
		dirs: map[string]bool{
			"/host": true,
		},
		files: map[string][]byte{
			"/proc/self/cgroup": []byte("0::/docker/" + id),
			metaPath:            []byte(`{"Config":{"Labels":{"com.docker.compose.project.working_dir":"/srv/stack"}}}`),
		},
	}
	wd, err := DetectComposeWorkingDir(env)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if wd != "/srv/stack" {
		t.Fatalf("wd=%s", wd)
	}
}

func TestPrepareHostValidation(t *testing.T) {
	t.Parallel()
	env := []string{"task.schedule=1h", "task.cmd=true", "task.run=host"}
	cfg, err := config.ParseEnvironment(env)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	_, err = Prepare(&cfg, fakeEnv{uid: 1000, dirs: map[string]bool{"/host": true}, files: map[string][]byte{}})
	if err == nil {
		t.Fatalf("expected root error")
	}
	if !strings.Contains(err.Error(), "require root for chroot") {
		t.Fatalf("unexpected root error: %v", err)
	}

	_, err = Prepare(&cfg, fakeEnv{uid: 0, dirs: map[string]bool{}, files: map[string][]byte{}})
	if err == nil {
		t.Fatalf("expected host mount error")
	}
	if !strings.Contains(err.Error(), "require /host mount") {
		t.Fatalf("unexpected host mount error: %v", err)
	}
}

func TestPrepareComposeValidation(t *testing.T) {
	t.Parallel()
	id := "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	metaPath := filepath.Join("/host/var/lib/docker/containers", id, "config.v2.json")
	envCfg := []string{"task.schedule=1h", "task.cmd=true", "task.run=compose"}
	cfg, err := config.ParseEnvironment(envCfg)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	env := fakeEnv{
		uid:  0,
		dirs: map[string]bool{"/host": true},
		files: map[string][]byte{
			"/host/bin/bash":    []byte(""),
			"/proc/self/cgroup": []byte("0::/docker/" + id),
			metaPath:            []byte(`{"Config":{"Labels":{"com.docker.compose.project.working_dir":"/srv/stack"}}}`),
		},
	}
	prepared, err := Prepare(&cfg, env)
	if err != nil {
		t.Fatalf("prepare: %v", err)
	}
	if prepared.ComposeDir != "/srv/stack" {
		t.Fatalf("compose dir=%s", prepared.ComposeDir)
	}
}

func TestPrepareHostShellMissing(t *testing.T) {
	t.Parallel()
	envCfg := []string{
		"task.schedule=1h",
		"task.cmd=true",
		"task.run=host",
		"task.shell=/bin/does-not-exist -lc",
	}
	cfg, err := config.ParseEnvironment(envCfg)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	_, err = Prepare(&cfg, fakeEnv{
		uid:   0,
		dirs:  map[string]bool{"/host": true},
		files: map[string][]byte{},
	})
	if err == nil {
		t.Fatalf("expected shell missing error")
	}
	if !strings.Contains(err.Error(), `shell "/bin/does-not-exist" not found on host`) {
		t.Fatalf("unexpected shell missing error: %v", err)
	}
}

func TestPrepareComposeMissingWorkingDirLabel(t *testing.T) {
	t.Parallel()
	id := "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	metaPath := filepath.Join("/host/var/lib/docker/containers", id, "config.v2.json")
	envCfg := []string{"task.schedule=1h", "task.cmd=true", "task.run=compose"}
	cfg, err := config.ParseEnvironment(envCfg)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	env := fakeEnv{
		uid:  0,
		dirs: map[string]bool{"/host": true},
		files: map[string][]byte{
			"/host/bin/bash":    []byte(""),
			"/proc/self/cgroup": []byte("0::/docker/" + id),
			metaPath:            []byte(`{"Config":{"Labels":{}}}`),
		},
	}
	_, err = Prepare(&cfg, env)
	if err == nil {
		t.Fatalf("expected missing label error")
	}
	if !strings.Contains(err.Error(), "missing docker label com.docker.compose.project.working_dir") {
		t.Fatalf("unexpected missing label error: %v", err)
	}
}

func TestPrepareAllowsUserInContainerMode(t *testing.T) {
	t.Parallel()
	envCfg := []string{
		"task.schedule=1h",
		"task.cmd=true",
		"task.run=container",
		"task.user=1000:1000",
	}
	cfg, err := config.ParseEnvironment(envCfg)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	prepared, err := Prepare(&cfg, fakeEnv{uid: 0, dirs: map[string]bool{}, files: map[string][]byte{}})
	if err != nil {
		t.Fatalf("expected container user to be allowed, got: %v", err)
	}
	if prepared.ComposeDir != "" {
		t.Fatalf("unexpected compose dir: %q", prepared.ComposeDir)
	}
}
