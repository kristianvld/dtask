package notify

import (
	"context"
	"fmt"
	"net/url"
	"os/exec"
	"path/filepath"
	"strings"
)

var attachmentSupportedSchemes = map[string]bool{
	"discord":  true,
	"gmail":    true,
	"gotify":   true,
	"ifttt":    true,
	"mailto":   true,
	"matrix":   true,
	"ntfy":     true,
	"pushover": true,
	"slack":    true,
	"telegram": true,
}

func ValidateURL(raw string) error {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	u, err := url.Parse(raw)
	if err != nil {
		return fmt.Errorf("invalid notify_url: %w", err)
	}
	if u.Scheme == "" {
		return fmt.Errorf("invalid notify_url: missing scheme")
	}
	return nil
}

func SupportsAttachment(raw string) (bool, error) {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return false, fmt.Errorf("invalid notify_url: %w", err)
	}
	if u.Scheme == "" {
		return false, fmt.Errorf("invalid notify_url: missing scheme")
	}
	_, ok := attachmentSupportedSchemes[strings.ToLower(u.Scheme)]
	return ok, nil
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
	if b, err := exec.LookPath("apprise-go"); err == nil {
		return &CommandSender{binary: b}, nil
	}
	if b, err := exec.LookPath("apprise"); err == nil {
		return &CommandSender{binary: b}, nil
	}
	return nil, fmt.Errorf("neither apprise-go nor apprise binary found in PATH")
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
