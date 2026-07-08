import SwiftUI
import TownCrierDomain

/// A bottom sheet listing the planning applications stacked at one location —
/// the disambiguation list shown when a coincident ("unsplittable") map cluster
/// is tapped (GH#722, and GH#877 on the anonymous map). Zoom can never separate
/// such members, so instead of zooming the user picks from this list; each row
/// opens that application's existing summary sheet via the injected
/// ``onSelect`` closure, exactly as a single-pin tap does.
///
/// Presentation-only — no concrete view model dependency — so both the
/// authenticated map (``MapViewModel/selectFromStack(_:)``) and the anonymous
/// map (``AnonymousMapViewModel/selectFromStack(_:)``) can present the same
/// sheet. The caller's `onSelect` is expected to hand off through its own
/// view model's dismiss-then-present pattern so the list dismisses before the
/// summary presents — the two sheets are never on screen at once (SwiftUI's
/// two-sheets race).
struct StackedApplicationsSheet: View {
  let stacked: StackedApplications
  let onSelect: (PlanningApplication) -> Void

  var body: some View {
    VStack(alignment: .leading, spacing: 0) {
      header
      Divider().overlay(Color.tcBorder)
      ScrollView {
        LazyVStack(spacing: 0) {
          ForEach(stacked.applications) { application in
            Button {
              onSelect(application)
            } label: {
              StackedApplicationRow(application: application)
                .contentShape(Rectangle())
            }
            .buttonStyle(.plain)
            .accessibilityLabel(Self.accessibilityLabel(for: application))
            .accessibilityHint("Opens this application's summary")

            if application.id != stacked.applications.last?.id {
              Divider()
                .overlay(Color.tcBorder)
                .padding(.leading, TCSpacing.medium)
            }
          }
        }
      }
    }
    .frame(maxWidth: .infinity, alignment: .leading)
    .background(Color.tcSurfaceElevated)
    .presentationDetents([.medium, .large])
    .presentationDragIndicator(.visible)
  }

  private var header: some View {
    VStack(alignment: .leading, spacing: TCSpacing.extraSmall) {
      Text("Applications at this location")
        .font(TCTypography.displaySmall)
        .foregroundStyle(Color.tcTextPrimary)
      Text("\(stacked.applications.count) applications share this address")
        .font(TCTypography.caption)
        .foregroundStyle(Color.tcTextSecondary)
    }
    .padding(TCSpacing.medium)
    .frame(maxWidth: .infinity, alignment: .leading)
    .accessibilityElement(children: .combine)
  }

  /// A flat, screen-reader-friendly summary of a row, so VoiceOver announces the
  /// status, reference, and address in one phrase rather than three fragments.
  static func accessibilityLabel(for application: PlanningApplication) -> String {
    "\(application.status.displayLabel), reference \(application.reference.value), \(application.address)"
  }
}

/// One row of the stacked-applications list. Borrows the ``ApplicationListRow``
/// vocabulary — a status-coloured pill, the description, and the address — and
/// adds the case reference so coincident applications at the same address are
/// distinguishable. A leading chevron signals the row is tappable.
private struct StackedApplicationRow: View {
  let application: PlanningApplication

  var body: some View {
    HStack(alignment: .top, spacing: TCSpacing.small) {
      VStack(alignment: .leading, spacing: TCSpacing.small) {
        HStack {
          ApplicationStatusPill(status: application.status)
          Spacer()
          Text(application.reference.value)
            .font(TCTypography.caption)
            .foregroundStyle(Color.tcTextSecondary)
        }

        Text(application.description)
          .font(TCTypography.headline)
          .foregroundStyle(Color.tcTextPrimary)
          .lineLimit(2)
          .multilineTextAlignment(.leading)

        Text(application.address)
          .font(TCTypography.caption)
          .foregroundStyle(Color.tcTextSecondary)
          .lineLimit(1)
      }

      Image(systemName: "chevron.forward")
        .font(TCTypography.caption)
        .foregroundStyle(Color.tcTextTertiary)
        .accessibilityHidden(true)
    }
    .padding(.horizontal, TCSpacing.medium)
    .padding(.vertical, TCSpacing.small)
    .frame(maxWidth: .infinity, alignment: .leading)
  }
}
