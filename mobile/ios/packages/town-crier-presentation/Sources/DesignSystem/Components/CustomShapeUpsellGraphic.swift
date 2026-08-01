import Foundation
import SwiftUI

/// Decorative onboarding graphic contrasting the default **circle** watch
/// zone with a **custom shape** (polygon) watch zone, to visually explain the
/// paid "draw a custom shape instead of a circle" entitlement (GH#1031).
///
/// Drawn entirely in code with SwiftUI `Path`, never a raster asset — ADR
/// 0040 single-sources design tokens, and a bitmap can't follow the
/// light/dark/OLED theme. The muted, dashed circle reads as the existing,
/// unremarkable default; the solid `tcAmber` polygon reads as the premium
/// capability, matching the app's convention that `tcAmber` marks brand and
/// interactive emphasis. Purely presentational: no ViewModel, no external
/// state — every color and spacing value comes from a design token, so the
/// graphic needs no wiring to render correctly in any theme.
///
/// Not yet wired into any screen — that's a separate bead (onboarding step
/// placement).
public struct CustomShapeUpsellGraphic: View {
  private static let tileSize: CGFloat = 96

  public init() {}

  public var body: some View {
    HStack(spacing: TCSpacing.large) {
      Circle()
        .stroke(Color.tcTextTertiary, style: StrokeStyle(lineWidth: 2, dash: [6, 4]))
        .frame(width: Self.tileSize, height: Self.tileSize)

      Image(systemName: "arrow.right")
        .font(TCTypography.body)
        .foregroundStyle(Color.tcTextSecondary)

      CustomShapePolygon()
        .stroke(Color.tcAmber, style: StrokeStyle(lineWidth: 2, lineJoin: .round))
        .frame(width: Self.tileSize, height: Self.tileSize)
    }
    .padding(TCSpacing.large)
    .accessibilityElement(children: .ignore)
    .accessibilityLabel(
      "Illustration: a plain circle watch zone next to a custom, hand-drawn shape watch zone"
    )
  }
}

/// The irregular polygon ("blob") half of ``CustomShapeUpsellGraphic`` — a
/// closed shape that reads as hand-drawn rather than a regular polygon, to
/// contrast visually with the perfectly round circle beside it.
struct CustomShapePolygon: Shape {
  func path(in rect: CGRect) -> Path {
    let vertices = CustomShapePolygonGeometry.vertices(in: rect)
    var path = Path()
    guard let first = vertices.first else { return path }

    path.move(to: first)
    for vertex in vertices.dropFirst() {
      path.addLine(to: vertex)
    }
    path.closeSubpath()
    return path
  }
}

/// Pure geometry for ``CustomShapePolygon``, factored out of the view so the
/// point generation is unit-testable without instantiating SwiftUI.
///
/// Vertices are placed at evenly-spaced angles around the centre of the
/// given rect, each pulled in from a shared base radius by a fixed,
/// hand-picked multiplier no greater than `1.0` (so every vertex stays
/// inscribed within the rect passed to it — no overflow into a neighbouring
/// element). The multipliers are deliberately irregular (not a smooth curve
/// or true randomness) so the outline reads as a hand-drawn boundary rather
/// than a regular polygon — deterministic and simple, since this is a
/// decorative graphic rather than data-driven geometry.
enum CustomShapePolygonGeometry {
  static let radiusMultipliers: [CGFloat] = [0.9, 0.6, 1.0, 0.55, 0.85, 0.5, 0.95, 0.65]

  static func vertices(in rect: CGRect) -> [CGPoint] {
    let center = CGPoint(x: rect.midX, y: rect.midY)
    let baseRadius = min(rect.width, rect.height) / 2
    let count = radiusMultipliers.count

    return radiusMultipliers.enumerated().map { index, multiplier in
      let angle = (CGFloat(index) / CGFloat(count)) * 2 * .pi
      let radius = baseRadius * multiplier
      return CGPoint(
        x: center.x + radius * cos(angle),
        y: center.y + radius * sin(angle)
      )
    }
  }
}
