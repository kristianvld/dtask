package notify

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

const appriseProbeScript = `
import json
import sys

try:
    import apprise
except Exception as exc:
    print(json.dumps({"error": f"failed to import apprise: {exc}"}))
    raise SystemExit(3)

url = sys.argv[1].strip()
if not url:
    print(json.dumps({"valid": True, "attachment_supported": False}))
    raise SystemExit(0)

try:
    plugin = apprise.Apprise.instantiate(url)
except Exception as exc:
    print(json.dumps({"valid": False, "attachment_supported": False, "error": str(exc)}))
    raise SystemExit(0)

print(json.dumps({
    "valid": bool(plugin),
    "attachment_supported": bool(getattr(plugin, "attachment_support", False)) if plugin else False
}))
`

type appriseProbeResult struct {
	Valid               bool   `json:"valid"`
	AttachmentSupported bool   `json:"attachment_supported"`
	Error               string `json:"error,omitempty"`
}

var inspectURL = inspectURLWithApprise

func ValidateURL(raw string) error {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	res, err := inspectURL(raw)
	if err != nil {
		return fmt.Errorf("invalid notify_url: %w", err)
	}
	if !res.Valid {
		return fmt.Errorf("invalid notify_url: unsupported or malformed apprise URL")
	}
	return nil
}

func SupportsAttachment(raw string) (bool, error) {
	res, err := inspectURL(strings.TrimSpace(raw))
	if err != nil {
		return false, fmt.Errorf("invalid notify_url: %w", err)
	}
	if !res.Valid {
		return false, fmt.Errorf("invalid notify_url: unsupported or malformed apprise URL")
	}
	return res.AttachmentSupported, nil
}

func inspectURLWithApprise(raw string) (appriseProbeResult, error) {
	python, err := exec.LookPath("python3")
	if err != nil {
		return appriseProbeResult{}, fmt.Errorf("python3 not found in PATH")
	}

	cmd := exec.Command(python, "-c", appriseProbeScript, raw)
	cmd.Env = os.Environ()
	out, err := cmd.CombinedOutput()
	if err != nil {
		payload := strings.TrimSpace(string(out))
		if payload == "" {
			payload = err.Error()
		}
		return appriseProbeResult{}, fmt.Errorf("apprise probe failed: %s", payload)
	}

	var res appriseProbeResult
	if err := json.Unmarshal(out, &res); err != nil {
		return appriseProbeResult{}, fmt.Errorf("failed to parse apprise probe output: %w", err)
	}
	if res.Error != "" {
		return appriseProbeResult{}, fmt.Errorf("apprise probe failed: %s", res.Error)
	}
	return res, nil
}

type Request struct {
	URL         string
	Title       string
	Body        string
	Attachments []string
}

type Sender interface {
	Send(ctx context.Context, req Request) error
}

type CommandSender struct {
	binary string
}

func NewCommandSender() (*CommandSender, error) {
	if b, err := exec.LookPath("apprise"); err == nil {
		return &CommandSender{binary: b}, nil
	}
	return nil, fmt.Errorf("apprise binary not found in PATH")
}

func (s *CommandSender) Send(ctx context.Context, req Request) error {
	if strings.TrimSpace(req.URL) == "" {
		return nil
	}

	args := []string{"-t", req.Title, "-b", req.Body}
	for _, a := range req.Attachments {
		if strings.TrimSpace(a) == "" {
			continue
		}
		args = append(args, "--attach", a)
	}
	args = append(args, req.URL)

	cmd := exec.CommandContext(ctx, s.binary, args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		name := filepath.Base(s.binary)
		return fmt.Errorf("%s send failed: %w: %s", name, err, strings.TrimSpace(string(out)))
	}
	return nil
}
