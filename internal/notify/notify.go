package notify

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
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

const (
	defaultNotifyType = "info"
	notifyTypeSuccess = "success"
	notifyTypeWarning = "warning"
	notifyTypeFailure = "failure"
)

const appriseSendScript = `
import json
import sys
from urllib.parse import parse_qs, urlencode, urlsplit, urlunsplit

try:
    import apprise
except Exception as exc:
    print(json.dumps({"error": f"failed to import apprise: {exc}"}))
    raise SystemExit(3)

payload = json.loads(sys.argv[1])
url = (payload.get("url") or "").strip()

if not url:
    print(json.dumps({"ok": True}))
    raise SystemExit(0)

parsed = urlsplit(url)
params = parse_qs(parsed.query, keep_blank_values=True)

def take(name, default_value):
    values = params.pop(name, None)
    if not values:
        return default_value
    value = (values[-1] or "").strip()
    return value if value else default_value

image_url_logo = take("image_url_logo", "https://raw.githubusercontent.com/kristianvld/dtask/main/docs/public/logo.png")
image_url_mask = take("image_url_mask", image_url_logo)

asset = apprise.AppriseAsset(
    app_id=take("app_id", "dtask"),
    app_desc=take("app_desc", "dtask scheduled task runner"),
    app_url=take("app_url", "https://github.com/kristianvld/dtask"),
    image_url_logo=image_url_logo,
    image_url_mask=image_url_mask,
)

clean_url = urlunsplit((
    parsed.scheme,
    parsed.netloc,
    parsed.path,
    urlencode(params, doseq=True),
    parsed.fragment,
))

apobj = apprise.Apprise(asset=asset)
if not apobj.add(clean_url):
    print(json.dumps({"error": "unsupported or malformed apprise URL"}))
    raise SystemExit(2)

notify_type = (payload.get("notify_type") or "info").strip().lower()
notify_type_map = {
    "info": apprise.NotifyType.INFO,
    "success": apprise.NotifyType.SUCCESS,
    "warning": apprise.NotifyType.WARNING,
    "failure": apprise.NotifyType.FAILURE,
}
ok = apobj.notify(
    title=payload.get("title", ""),
    body=payload.get("body", ""),
    attach=payload.get("attachments") or None,
    notify_type=notify_type_map.get(notify_type, apprise.NotifyType.INFO),
)
if not ok:
    print(json.dumps({"error": "apprise failed to send notification"}))
    raise SystemExit(1)

print(json.dumps({"ok": True}))
`

type appriseProbeResult struct {
	Valid               bool   `json:"valid"`
	AttachmentSupported bool   `json:"attachment_supported"`
	Error               string `json:"error,omitempty"`
}

type appriseSendResult struct {
	OK    bool   `json:"ok"`
	Error string `json:"error,omitempty"`
}

type appriseSendPayload struct {
	URL         string   `json:"url"`
	Title       string   `json:"title"`
	Body        string   `json:"body"`
	Attachments []string `json:"attachments,omitempty"`
	NotifyType  string   `json:"notify_type,omitempty"`
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
	NotifyType  string
}

type Sender interface {
	Send(ctx context.Context, req Request) error
}

type CommandSender struct {
	python string
}

func NewCommandSender() (*CommandSender, error) {
	python, err := exec.LookPath("python3")
	if err != nil {
		return nil, fmt.Errorf("python3 not found in PATH")
	}
	_, err = inspectURLWithApprise("")
	if err != nil {
		return nil, err
	}
	return &CommandSender{python: python}, nil
}

func (s *CommandSender) Send(ctx context.Context, req Request) error {
	if strings.TrimSpace(req.URL) == "" {
		return nil
	}

	payload := appriseSendPayload{
		URL:        req.URL,
		Title:      req.Title,
		Body:       req.Body,
		NotifyType: normalizeNotifyType(req.NotifyType),
	}
	for _, a := range req.Attachments {
		if strings.TrimSpace(a) == "" {
			continue
		}
		payload.Attachments = append(payload.Attachments, a)
	}

	b, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to encode notify payload: %w", err)
	}

	cmd := exec.CommandContext(ctx, s.python, "-c", appriseSendScript, string(b))
	cmd.Env = os.Environ()
	out, err := cmd.CombinedOutput()
	if err != nil {
		payload := strings.TrimSpace(string(out))
		if payload == "" {
			payload = err.Error()
		}
		return fmt.Errorf("apprise send failed: %s", payload)
	}

	var res appriseSendResult
	if err := json.Unmarshal(out, &res); err != nil {
		return fmt.Errorf("failed to parse apprise send output: %w", err)
	}
	if !res.OK {
		if res.Error == "" {
			res.Error = "unknown error"
		}
		return fmt.Errorf("apprise send failed: %s", res.Error)
	}
	return nil
}

func normalizeNotifyType(v string) string {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case notifyTypeSuccess:
		return notifyTypeSuccess
	case notifyTypeWarning:
		return notifyTypeWarning
	case notifyTypeFailure:
		return notifyTypeFailure
	default:
		return defaultNotifyType
	}
}
