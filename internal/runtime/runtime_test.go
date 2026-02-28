package runtime

import (
	"os"
	"path/filepath"
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

	if _, err := Prepare(&cfg, fakeEnv{uid: 1000, dirs: map[string]bool{"/host": true}, files: map[string][]byte{}}); err == nil {
		t.Fatalf("expected root error")
	}

	if _, err := Prepare(&cfg, fakeEnv{uid: 0, dirs: map[string]bool{}, files: map[string][]byte{}}); err == nil {
		t.Fatalf("expected host mount error")
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
