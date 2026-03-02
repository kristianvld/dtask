package notify

import (
	"strings"
	"testing"
)

func TestValidateURL(t *testing.T) {
	oldInspect := inspectURL
	inspectURL = func(raw string) (appriseProbeResult, error) {
		switch raw {
		case "discord://webhook_id/webhook_token":
			return appriseProbeResult{Valid: true, AttachmentSupported: true}, nil
		case "://bad":
			return appriseProbeResult{Valid: false}, nil
		default:
			return appriseProbeResult{}, nil
		}
	}
	t.Cleanup(func() { inspectURL = oldInspect })

	if err := ValidateURL("discord://webhook_id/webhook_token"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if err := ValidateURL(" "); err != nil {
		t.Fatalf("empty should be valid")
	}
	if err := ValidateURL("://bad"); err == nil {
		t.Fatalf("expected error")
	}
}

func TestSupportsAttachment(t *testing.T) {
	oldInspect := inspectURL
	inspectURL = func(raw string) (appriseProbeResult, error) {
		switch raw {
		case "discord://x/y":
			return appriseProbeResult{Valid: true, AttachmentSupported: true}, nil
		case "custom://x":
			return appriseProbeResult{Valid: true, AttachmentSupported: false}, nil
		default:
			return appriseProbeResult{}, nil
		}
	}
	t.Cleanup(func() { inspectURL = oldInspect })

	ok, err := SupportsAttachment("discord://x/y")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ok {
		t.Fatalf("expected support")
	}
	ok, err = SupportsAttachment("custom://x")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ok {
		t.Fatalf("unexpected support")
	}
}

func TestAppriseSendScriptSupportsImageURLMask(t *testing.T) {
	t.Parallel()
	if !strings.Contains(appriseSendScript, `take("image_url_mask", image_url_logo)`) {
		t.Fatalf("apprise send script must default image_url_mask from image_url_logo")
	}
	if !strings.Contains(appriseSendScript, `image_url_mask=image_url_mask`) {
		t.Fatalf("apprise send script must set image_url_mask on AppriseAsset")
	}
}
