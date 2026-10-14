package seopage

import (
	"fmt"
	"html"
	"html/template"
	"strings"

	"rsc.io/qr"
)

const qrQuietZone = 4

// qrSVG renders text as an inline SVG QR code (error correction M, quiet zone
// 4) drawn as one path of unit squares, the same shape the prerendered pages use.
func qrSVG(text, ariaLabel string) (template.HTML, error) {
	code, err := qr.Encode(text, qr.M)
	if err != nil {
		return "", fmt.Errorf("encode QR code: %w", err)
	}
	box := code.Size + qrQuietZone*2
	var path strings.Builder
	for row := range code.Size {
		for col := range code.Size {
			if code.Black(col, row) {
				fmt.Fprintf(&path, "M%d %dh1v1h-1z", col+qrQuietZone, row+qrQuietZone)
			}
		}
	}
	return template.HTML(fmt.Sprintf( //nolint:gosec // every interpolated value is numeric, escaped or a module path
		`<svg class="qr" role="img" aria-label="%s" viewBox="0 0 %d %d" shape-rendering="crispEdges" xmlns="http://www.w3.org/2000/svg">`+
			`<rect width="%d" height="%d" fill="#FFFFFF"/><path d="%s" fill="#000000"/></svg>`,
		html.EscapeString(ariaLabel), box, box, box, box, path.String())), nil
}
