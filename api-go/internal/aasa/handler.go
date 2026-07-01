// Package aasa serves the Apple App Site Association document on the public
// share host (#738, Slice 3). Apple's daemon fetches
// GET /.well-known/apple-app-site-association — with no ".json" extension on the
// path — and REQUIRES the response carry Content-Type application/json. The
// document declares the Universal Links applinks component for the share-page
// path /a/*, associated with the app's App ID (TeamID.BundleID), so a shared
// link https://share.towncrierapp.uk/a/{slug}/{ref} opens the installed app.
//
// The endpoint is stateless (no store) and emits only the fixed, public
// association document, so wiring registers it unconditionally alongside the
// other stateless routes.
package aasa

import (
	"encoding/json"
	"log/slog"
	"net/http"
)

// wellKnownPath is the AASA location Apple fetches. It deliberately carries no
// ".json" extension: the Content-Type identifies the document, not the path.
const wellKnownPath = "/.well-known/apple-app-site-association"

// pathComponent is the applinks path component the share pages live under. A
// shared link .../a/{slug}/{ref} matches /a/*.
const pathComponent = "/a/*"

// document, applinks, detail and componentEntry model the AASA JSON so it is
// marshalled from typed data (always valid) rather than string-built. The
// componentEntry's "/" key is the applinks path-pattern field Apple expects.
type document struct {
	Applinks applinks `json:"applinks"`
}

type applinks struct {
	Details []detail `json:"details"`
}

type detail struct {
	AppIDs     []string         `json:"appIDs"`
	Components []componentEntry `json:"components"`
}

type componentEntry struct {
	Path string `json:"/"`
}

type handler struct {
	body   []byte
	logger *slog.Logger
}

// Routes registers the anonymous AASA endpoint for the given App ID. Keep the
// pattern in lockstep with the anonymousPatterns entry in cmd/api/wiring.go —
// the auth middleware keys on the exact registered pattern string.
//
// The document is marshalled once at wiring time so the handler does no per-
// request work. Marshalling a fixed-shape struct of strings cannot fail; on the
// impossible error the route is simply not registered (it then falls to the
// deny fallback) rather than panicking during boot.
func Routes(mux *http.ServeMux, appID string, logger *slog.Logger) {
	body, err := json.Marshal(document{
		Applinks: applinks{
			Details: []detail{{
				AppIDs:     []string{appID},
				Components: []componentEntry{{Path: pathComponent}},
			}},
		},
	})
	if err != nil {
		logger.Error("aasa: marshal association document", "error", err)
		return
	}
	h := &handler{body: body, logger: logger}
	mux.HandleFunc("GET "+wellKnownPath, h.serve)
}

func (h *handler) serve(w http.ResponseWriter, r *http.Request) {
	// Apple's daemon requires exactly application/json (no charset parameter).
	w.Header().Set("Content-Type", "application/json")
	if _, err := w.Write(h.body); err != nil {
		h.logger.ErrorContext(r.Context(), "write aasa response", "error", err)
	}
}
