package notifications

import "testing"

func TestDigestNotification_HasSavedSource(t *testing.T) {
	t.Parallel()
	if !(DigestNotification{Sources: "Zone, Saved"}).HasSavedSource() {
		t.Error("sources should include Saved")
	}
	if (DigestNotification{}).HasSavedSource() {
		t.Error("empty sources should not have Saved source")
	}
}

func TestDigestNotification_MarkEmailSent(t *testing.T) {
	t.Parallel()
	n := DigestNotification{ID: "n-3", EmailSent: false}
	n.MarkEmailSent()
	if !n.EmailSent {
		t.Fatal("MarkEmailSent should set EmailSent true")
	}
}
