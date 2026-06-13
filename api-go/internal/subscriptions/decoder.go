package subscriptions

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// PayloadError signals a decoded Apple payload that is malformed or missing a
// required field. It is the Go analog of the .NET ArgumentException the
// decoders and the bundle-id check raise — the verify/webhook endpoints map it
// to a 400 invalid_transaction_payload / invalid_notification_payload, whereas
// a *JWSVerificationError maps to a 401.
type PayloadError struct {
	Message string
}

func (e *PayloadError) Error() string { return e.Message }

// DecodedTransaction is the decoded JWSTransactionDecodedPayload (StoreKit 2).
// Dates are converted from Apple's Unix epoch milliseconds to UTC time.
type DecodedTransaction struct {
	TransactionID         string
	OriginalTransactionID string
	ProductID             string
	BundleID              string
	PurchaseDate          time.Time
	ExpiresDate           time.Time
	Environment           string
}

// DecodedNotification is the decoded App Store Server Notification v2
// (responseBodyV2DecodedPayload). The signed JWS strings for the transaction
// and renewal info are nested under data; Subtype and SignedRenewalInfo are
// optional (empty when absent).
type DecodedNotification struct {
	NotificationType      string
	Subtype               string
	NotificationUUID      string
	SignedTransactionInfo string
	SignedRenewalInfo     string
}

type appleTransactionPayload struct {
	TransactionID         string `json:"transactionId"`
	OriginalTransactionID string `json:"originalTransactionId"`
	ProductID             string `json:"productId"`
	BundleID              string `json:"bundleId"`
	PurchaseDate          int64  `json:"purchaseDate"`
	ExpiresDate           int64  `json:"expiresDate"`
	Environment           string `json:"environment"`
}

type appleNotificationPayload struct {
	NotificationType string                 `json:"notificationType"`
	Subtype          string                 `json:"subtype"`
	NotificationUUID string                 `json:"notificationUUID"`
	Data             *appleNotificationData `json:"data"`
}

type appleNotificationData struct {
	SignedTransactionInfo string `json:"signedTransactionInfo"`
	SignedRenewalInfo     string `json:"signedRenewalInfo"`
}

// DecodeTransaction maps the verified JWS transaction JSON onto a
// DecodedTransaction, mirroring .NET TransactionDecoder including its exact
// error messages.
func DecodeTransaction(jsonStr string) (DecodedTransaction, error) {
	if strings.TrimSpace(jsonStr) == "" {
		return DecodedTransaction{}, &PayloadError{Message: "The transaction JSON is empty."}
	}

	var p appleTransactionPayload
	if err := json.Unmarshal([]byte(jsonStr), &p); err != nil {
		return DecodedTransaction{}, &PayloadError{Message: "The transaction JSON is malformed."}
	}

	for _, req := range []struct{ value, field string }{
		{p.TransactionID, "transactionId"},
		{p.OriginalTransactionID, "originalTransactionId"},
		{p.ProductID, "productId"},
		{p.BundleID, "bundleId"},
		{p.Environment, "environment"},
	} {
		if req.value == "" {
			return DecodedTransaction{}, missingField("transaction", req.field)
		}
	}

	return DecodedTransaction{
		TransactionID:         p.TransactionID,
		OriginalTransactionID: p.OriginalTransactionID,
		ProductID:             p.ProductID,
		BundleID:              p.BundleID,
		PurchaseDate:          time.UnixMilli(p.PurchaseDate).UTC(),
		ExpiresDate:           time.UnixMilli(p.ExpiresDate).UTC(),
		Environment:           p.Environment,
	}, nil
}

// DecodeNotification maps the verified outer JWS notification JSON onto a
// DecodedNotification, mirroring .NET NotificationDecoder including its exact
// error messages.
func DecodeNotification(jsonStr string) (DecodedNotification, error) {
	if strings.TrimSpace(jsonStr) == "" {
		return DecodedNotification{}, &PayloadError{Message: "The notification JSON is empty."}
	}

	var p appleNotificationPayload
	if err := json.Unmarshal([]byte(jsonStr), &p); err != nil {
		return DecodedNotification{}, &PayloadError{Message: "The notification JSON is malformed."}
	}

	if p.NotificationType == "" {
		return DecodedNotification{}, missingField("notification", "notificationType")
	}
	if p.NotificationUUID == "" {
		return DecodedNotification{}, missingField("notification", "notificationUUID")
	}
	var signedTxn, signedRenewal string
	if p.Data != nil {
		signedTxn = p.Data.SignedTransactionInfo
		signedRenewal = p.Data.SignedRenewalInfo
	}
	if signedTxn == "" {
		return DecodedNotification{}, missingField("notification", "data.signedTransactionInfo")
	}

	return DecodedNotification{
		NotificationType:      p.NotificationType,
		Subtype:               p.Subtype,
		NotificationUUID:      p.NotificationUUID,
		SignedTransactionInfo: signedTxn,
		SignedRenewalInfo:     signedRenewal,
	}, nil
}

func missingField(subject, field string) error {
	return &PayloadError{Message: fmt.Sprintf("The %s JSON is missing the required '%s' field.", subject, field)}
}
