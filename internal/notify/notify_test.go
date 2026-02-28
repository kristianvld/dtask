package notify

import "testing"

func TestValidateURL(t *testing.T) {
	t.Parallel()
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
	t.Parallel()
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
