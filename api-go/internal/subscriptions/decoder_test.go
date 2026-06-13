package subscriptions

import (
	"errors"
	"testing"
	"time"
)

func TestDecodeTransaction_Valid(t *testing.T) {
	t.Parallel()
	const purchaseMs = 1_700_000_000_000
	const expiresMs = 1_702_592_000_000
	json := `{
		"transactionId":"txn-1",
		"originalTransactionId":"orig-1",
		"productId":"uk.towncrierapp.pro.monthly",
		"bundleId":"uk.towncrierapp.mobile",
		"purchaseDate":1700000000000,
		"expiresDate":1702592000000,
		"environment":"Production"
	}`

	got, err := DecodeTransaction(json)
	if err != nil {
		t.Fatalf("DecodeTransaction: %v", err)
	}
	if got.TransactionID != "txn-1" || got.OriginalTransactionID != "orig-1" {
		t.Errorf("ids = %q/%q", got.TransactionID, got.OriginalTransactionID)
	}
	if got.ProductID != "uk.towncrierapp.pro.monthly" || got.BundleID != "uk.towncrierapp.mobile" {
		t.Errorf("product/bundle = %q/%q", got.ProductID, got.BundleID)
	}
	if got.Environment != "Production" {
		t.Errorf("environment = %q", got.Environment)
	}
	if want := time.UnixMilli(purchaseMs).UTC(); !got.PurchaseDate.Equal(want) {
		t.Errorf("purchaseDate = %v, want %v", got.PurchaseDate, want)
	}
	if want := time.UnixMilli(expiresMs).UTC(); !got.ExpiresDate.Equal(want) {
		t.Errorf("expiresDate = %v, want %v", got.ExpiresDate, want)
	}
}

func TestDecodeTransaction_Errors(t *testing.T) {
	t.Parallel()
	base := `{"transactionId":"t","originalTransactionId":"o","productId":"p","bundleId":"b","purchaseDate":1,"expiresDate":2,"environment":"e"}`
	_ = base
	tests := []struct {
		name    string
		json    string
		wantMsg string
	}{
		{"empty", "   ", "The transaction JSON is empty."},
		{"malformed", "{not json", "The transaction JSON is malformed."},
		{"missing transactionId", `{"originalTransactionId":"o","productId":"p","bundleId":"b","environment":"e"}`, "The transaction JSON is missing the required 'transactionId' field."},
		{"missing originalTransactionId", `{"transactionId":"t","productId":"p","bundleId":"b","environment":"e"}`, "The transaction JSON is missing the required 'originalTransactionId' field."},
		{"missing productId", `{"transactionId":"t","originalTransactionId":"o","bundleId":"b","environment":"e"}`, "The transaction JSON is missing the required 'productId' field."},
		{"missing bundleId", `{"transactionId":"t","originalTransactionId":"o","productId":"p","environment":"e"}`, "The transaction JSON is missing the required 'bundleId' field."},
		{"missing environment", `{"transactionId":"t","originalTransactionId":"o","productId":"p","bundleId":"b"}`, "The transaction JSON is missing the required 'environment' field."},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			_, err := DecodeTransaction(tc.json)
			requirePayloadError(t, err, tc.wantMsg)
		})
	}
}

func TestDecodeNotification_Valid(t *testing.T) {
	t.Parallel()
	json := `{
		"notificationType":"DID_RENEW",
		"subtype":"BILLING_RECOVERY",
		"notificationUUID":"uuid-1",
		"data":{"signedTransactionInfo":"jws.txn.sig","signedRenewalInfo":"jws.renew.sig"}
	}`

	got, err := DecodeNotification(json)
	if err != nil {
		t.Fatalf("DecodeNotification: %v", err)
	}
	if got.NotificationType != "DID_RENEW" || got.Subtype != "BILLING_RECOVERY" {
		t.Errorf("type/subtype = %q/%q", got.NotificationType, got.Subtype)
	}
	if got.NotificationUUID != "uuid-1" {
		t.Errorf("uuid = %q", got.NotificationUUID)
	}
	if got.SignedTransactionInfo != "jws.txn.sig" || got.SignedRenewalInfo != "jws.renew.sig" {
		t.Errorf("signed info = %q/%q", got.SignedTransactionInfo, got.SignedRenewalInfo)
	}
}

func TestDecodeNotification_OptionalFieldsAbsent(t *testing.T) {
	t.Parallel()
	// subtype and signedRenewalInfo are optional — a SUBSCRIBED with neither is valid.
	json := `{"notificationType":"SUBSCRIBED","notificationUUID":"uuid-2","data":{"signedTransactionInfo":"jws.txn"}}`

	got, err := DecodeNotification(json)
	if err != nil {
		t.Fatalf("DecodeNotification: %v", err)
	}
	if got.Subtype != "" || got.SignedRenewalInfo != "" {
		t.Errorf("expected empty optional fields, got subtype=%q renewal=%q", got.Subtype, got.SignedRenewalInfo)
	}
}

func TestDecodeNotification_Errors(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		json    string
		wantMsg string
	}{
		{"empty", " ", "The notification JSON is empty."},
		{"malformed", "{bad", "The notification JSON is malformed."},
		{"missing type", `{"notificationUUID":"u","data":{"signedTransactionInfo":"x"}}`, "The notification JSON is missing the required 'notificationType' field."},
		{"missing uuid", `{"notificationType":"SUBSCRIBED","data":{"signedTransactionInfo":"x"}}`, "The notification JSON is missing the required 'notificationUUID' field."},
		{"missing signed txn", `{"notificationType":"SUBSCRIBED","notificationUUID":"u","data":{}}`, "The notification JSON is missing the required 'data.signedTransactionInfo' field."},
		{"missing data", `{"notificationType":"SUBSCRIBED","notificationUUID":"u"}`, "The notification JSON is missing the required 'data.signedTransactionInfo' field."},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			_, err := DecodeNotification(tc.json)
			requirePayloadError(t, err, tc.wantMsg)
		})
	}
}

func requirePayloadError(t *testing.T, err error, wantMsg string) {
	t.Helper()
	var pe *PayloadError
	if !errors.As(err, &pe) {
		t.Fatalf("want *PayloadError, got %T (%v)", err, err)
	}
	if pe.Error() != wantMsg {
		t.Errorf("message = %q, want %q", pe.Error(), wantMsg)
	}
}
