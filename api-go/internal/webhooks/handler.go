package webhooks

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/AmyDe/town-crier/api-go/internal/auth"
	"github.com/AmyDe/town-crier/api-go/internal/httputil"
	"github.com/AmyDe/town-crier/api-go/internal/platform"
	"github.com/AmyDe/town-crier/api-go/internal/profiles"
)

// maxBodyBytes leaves room for a 2048 byte URL with every byte JSON-escaped.
const maxBodyBytes = 16 << 10

// Error codes are a client contract. Each is sent in "error" with a human
// "message".
const (
	notFoundCode    = "webhook_not_found"
	notFoundMessage = "You haven't set up a webhook."

	requiresProCode    = "webhook_requires_pro"
	requiresProMessage = "Webhooks need a Pro subscription."

	urlInvalidCode    = "webhook_url_invalid"
	urlInvalidMessage = "Enter a public https address. Only port 443 works."

	formatInvalidCode    = "webhook_format_invalid"
	formatInvalidMessage = "Choose JSON, Slack or Discord."

	testCooldownCode    = "webhook_test_cooldown"
	testCooldownMessage = "Wait 10 seconds before you send another test."
)

type endpointStore interface {
	ByUserID(ctx context.Context, userID string) (Endpoint, error)
	Create(ctx context.Context, e Endpoint) error
	Replace(ctx context.Context, userID, rawURL string, format Format, now time.Time) (Endpoint, error)
	RotateSecret(ctx context.Context, userID, secret string, now time.Time) error
	ClaimTest(ctx context.Context, userID string, now time.Time) (bool, error)
	DeleteAllByUserID(ctx context.Context, userID string) error
}

type handler struct {
	store    endpointStore
	profiles profileReader
	sender   messageSender
	newID    func() string
	now      func() time.Time
	logger   *slog.Logger
}

// Routes registers the /v1/me/webhook endpoints on mux. They need an
// authenticated subject in the request context. Setting, rotating and testing a
// webhook need an effective Pro tier. newID must return a new UUID. In
// production store is a *PostgresStore and sender a *Client.
func Routes(mux *http.ServeMux, store endpointStore, profiles profileReader, sender messageSender,
	newID func() string, now func() time.Time, logger *slog.Logger,
) {
	h := &handler{store: store, profiles: profiles, sender: sender, newID: newID, now: now, logger: logger}
	mux.HandleFunc("GET /v1/me/webhook", h.get)
	mux.HandleFunc("PUT /v1/me/webhook", h.put)
	mux.HandleFunc("DELETE /v1/me/webhook", h.delete)
	mux.HandleFunc("POST /v1/me/webhook/secret", h.rotateSecret)
	mux.HandleFunc("POST /v1/me/webhook/test", h.test)
}

// webhookResponse is the wire form of an endpoint. Secret is present only in
// the response that creates it. Paused means the user's tier is not Pro, so no
// alerts are queued.
type webhookResponse struct {
	URL               string          `json:"url"`
	Format            Format          `json:"format"`
	Status            Status          `json:"status"`
	DisabledReason    *DisabledReason `json:"disabledReason"`
	Paused            bool            `json:"paused"`
	CreatedAt         string          `json:"createdAt"`
	LastSuccessAt     *string         `json:"lastSuccessAt"`
	LastFailureAt     *string         `json:"lastFailureAt"`
	LastFailureReason *FailureReason  `json:"lastFailureReason"`
	LastFailureStatus *int            `json:"lastFailureStatus"`
	Secret            string          `json:"secret,omitempty"`
}

func responseOf(e Endpoint, paused bool) webhookResponse {
	return webhookResponse{
		URL:               e.URL,
		Format:            e.Format,
		Status:            e.Status,
		DisabledReason:    e.DisabledReason,
		Paused:            paused,
		CreatedAt:         formatInstant(e.CreatedAt),
		LastSuccessAt:     formatOptionalInstant(e.LastSuccessAt),
		LastFailureAt:     formatOptionalInstant(e.LastFailureAt),
		LastFailureReason: e.LastFailureReason,
		LastFailureStatus: e.LastFailureStatus,
	}
}

func formatOptionalInstant(t *time.Time) *string {
	if t == nil {
		return nil
	}
	return new(formatInstant(*t))
}

type putRequest struct {
	URL    string `json:"url"`
	Format Format `json:"format"`
}

type secretResponse struct {
	Secret string `json:"secret"`
}

type testResponse struct {
	Delivered     bool           `json:"delivered"`
	Status        *int           `json:"status"`
	FailureReason *FailureReason `json:"failureReason"`
}

func (h *handler) get(w http.ResponseWriter, r *http.Request) {
	userID := auth.Subject(r.Context())
	e, err := h.store.ByUserID(r.Context(), userID)
	if err != nil {
		h.storeError(w, r, "read webhook", err)
		return
	}
	pro, err := h.isPro(r.Context(), userID)
	if err != nil {
		h.serverError(w, r, "read profile", err)
		return
	}
	h.writeJSON(w, r, http.StatusOK, responseOf(e, !pro))
}

// put creates the user's webhook (201, with the secret) or replaces its URL and
// format (200, same secret), which also switches it on again.
func (h *handler) put(w http.ResponseWriter, r *http.Request) {
	userID := auth.Subject(r.Context())
	if !h.requirePro(w, r, userID) {
		return
	}
	var req putRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxBodyBytes)).Decode(&req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	if err := validateConfig(req.URL, req.Format); err != nil {
		if errors.Is(err, ErrInvalidFormat) {
			h.writeError(w, r, http.StatusBadRequest, formatInvalidCode, formatInvalidMessage)
			return
		}
		h.writeError(w, r, http.StatusBadRequest, urlInvalidCode, urlInvalidMessage)
		return
	}

	e, created, err := h.save(r.Context(), userID, req)
	if err != nil {
		h.serverError(w, r, "save webhook", err)
		return
	}
	resp := responseOf(e, false)
	if !created {
		h.writeJSON(w, r, http.StatusOK, resp)
		return
	}
	resp.Secret = e.Secret
	h.writeJSON(w, r, http.StatusCreated, resp)
}

func (h *handler) save(ctx context.Context, userID string, req putRequest) (Endpoint, bool, error) {
	e, err := h.store.Replace(ctx, userID, req.URL, req.Format, h.now().UTC())
	if !errors.Is(err, ErrNotFound) {
		return e, false, err
	}
	e, err = NewEndpoint(h.newID(), userID, req.URL, req.Format, h.now())
	if err != nil {
		return Endpoint{}, false, fmt.Errorf("build webhook: %w", err)
	}
	err = h.store.Create(ctx, e)
	if errors.Is(err, ErrEndpointExists) {
		// A concurrent request created it after the Replace above.
		e, err = h.store.Replace(ctx, userID, req.URL, req.Format, h.now().UTC())
		return e, false, err
	}
	return e, true, err
}

func (h *handler) delete(w http.ResponseWriter, r *http.Request) {
	if err := h.store.DeleteAllByUserID(r.Context(), auth.Subject(r.Context())); err != nil {
		h.serverError(w, r, "delete webhook", err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *handler) rotateSecret(w http.ResponseWriter, r *http.Request) {
	userID := auth.Subject(r.Context())
	if !h.requirePro(w, r, userID) {
		return
	}
	secret := NewSecret()
	if err := h.store.RotateSecret(r.Context(), userID, secret, h.now().UTC()); err != nil {
		h.storeError(w, r, "rotate webhook secret", err)
		return
	}
	h.writeJSON(w, r, http.StatusOK, secretResponse{Secret: secret})
}

// test sends the test event while the caller waits. It works on a switched-off
// webhook and leaves the failure counters alone.
func (h *handler) test(w http.ResponseWriter, r *http.Request) {
	userID := auth.Subject(r.Context())
	if !h.requirePro(w, r, userID) {
		return
	}
	e, err := h.store.ByUserID(r.Context(), userID)
	if err != nil {
		h.storeError(w, r, "read webhook", err)
		return
	}
	now := h.now().UTC()
	claimed, err := h.store.ClaimTest(r.Context(), userID, now)
	if err != nil {
		h.serverError(w, r, "claim webhook test", err)
		return
	}
	if !claimed {
		h.writeError(w, r, http.StatusTooManyRequests, testCooldownCode, testCooldownMessage)
		return
	}
	msg, err := testEventMessage(e, h.newID(), now)
	if err != nil {
		h.serverError(w, r, "build webhook test event", err)
		return
	}
	result := h.sender.Send(r.Context(), msg)
	if err := r.Context().Err(); err != nil {
		h.serverError(w, r, "send webhook test event", err)
		return
	}

	resp := testResponse{Delivered: result.Delivered()}
	if result.Status != 0 {
		resp.Status = &result.Status
	}
	if !result.Delivered() {
		resp.FailureReason = &result.Failure
	}
	h.writeJSON(w, r, http.StatusOK, resp)
}

func testEventMessage(e Endpoint, eventID string, now time.Time) (Message, error) {
	payload := BuildTestPayload(eventID, now)
	var body []byte
	var err error
	if e.Format == FormatSlack {
		body, err = SlackBody([]Payload{payload})
	} else {
		body, err = json.Marshal(payload)
	}
	if err != nil {
		return Message{}, fmt.Errorf("encode webhook test event: %w", err)
	}
	return Message{URL: e.URL, Secret: e.Secret, EventID: eventID, EventType: payload.Type, Body: body}, nil
}

// isPro reports whether the user's effective tier is Pro. A user with no
// profile is not.
func (h *handler) isPro(ctx context.Context, userID string) (bool, error) {
	profile, err := h.profiles.Get(ctx, userID)
	if errors.Is(err, profiles.ErrNotFound) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return profile != nil && profile.EffectiveTier(h.now()).IsPaidPro(), nil
}

// requirePro reports whether the request can continue. Otherwise it has
// written the response.
func (h *handler) requirePro(w http.ResponseWriter, r *http.Request, userID string) bool {
	pro, err := h.isPro(r.Context(), userID)
	if err != nil {
		h.serverError(w, r, "read profile", err)
		return false
	}
	if !pro {
		h.writeError(w, r, http.StatusForbidden, requiresProCode, requiresProMessage)
	}
	return pro
}

func (h *handler) storeError(w http.ResponseWriter, r *http.Request, op string, err error) {
	if errors.Is(err, ErrNotFound) {
		h.writeError(w, r, http.StatusNotFound, notFoundCode, notFoundMessage)
		return
	}
	h.serverError(w, r, op, err)
}

func (h *handler) serverError(w http.ResponseWriter, r *http.Request, op string, err error) {
	platform.RespondRequestError(w, r, h.logger, "webhook request failed", err, "op", op)
}

type errorResponse struct {
	Error   string `json:"error"`
	Message string `json:"message"`
}

func (h *handler) writeError(w http.ResponseWriter, r *http.Request, status int, code, message string) {
	h.writeJSON(w, r, status, errorResponse{Error: code, Message: message})
}

func (h *handler) writeJSON(w http.ResponseWriter, r *http.Request, status int, v any) {
	body, err := httputil.EncodeJSON(v)
	if err != nil {
		h.serverError(w, r, "encode response", err)
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if _, err := w.Write(body); err != nil { //nolint:gosec // G705 false positive: the body is JSON served as JSON
		h.logger.ErrorContext(r.Context(), "write response", "error", err)
	}
}
