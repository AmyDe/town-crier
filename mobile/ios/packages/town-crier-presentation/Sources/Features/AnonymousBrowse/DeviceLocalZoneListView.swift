import SwiftUI
import TownCrierDomain

/// The anonymous (pre-signup) Zones tab (GH#888): exactly one editable
/// device-local area. No toolbar "+", no swipe-to-delete — the on-device cap
/// is ``DeviceLocalZone/maxZoneCount`` == 1, and the only surviving mutation
/// is editing the single zone via a row tap. A persistent sign-up pitch sits
/// below the zone: with the add path gone, it is the only remaining route to
/// another area, so it is load-bearing rather than decorative — copy pitches
/// more areas AND alerts, and (like every anonymous CTA) never says
/// "instant", since instant alerts are a paid, server-enforced entitlement
/// (#868/#879 precedent).
public struct DeviceLocalZoneListView: View {
  /// Sign-up pitch copy not otherwise covered by an existing shared `Copy`
  /// enum. The eyebrow is new microcopy introduced by the upsell-card
  /// treatment (GH#857/#896); exposed here (rather than left inline) so it's
  /// unit-testable, matching this codebase's convention for CTA-bearing
  /// views (``AccountCTABanner/Copy``, ``DeviceLocalZoneSignUpCTAView/Copy``).
  enum Copy {
    static let eyebrow = "Free Account"
  }

  @StateObject private var viewModel: DeviceLocalZoneListViewModel

  public init(viewModel: DeviceLocalZoneListViewModel) {
    _viewModel = StateObject(wrappedValue: viewModel)
  }

  public var body: some View {
    List {
      mastheadRow
      if let zone = viewModel.zones.first {
        DeviceLocalZoneRow(zone: zone) { viewModel.requestAlertsSignUp() }
          .cardRowInsets()
          .contentShape(Rectangle())
          .onTapGesture { viewModel.editZone(zone) }
        signUpPitchSection
      } else {
        emptyState
      }
    }
    .scrollContentBackground(.hidden)
    .background(Color.tcBackground)
    .navigationTitle("Zones")
    .mastheadNavigationBar()
    .task {
      viewModel.load()
    }
    .sheet(item: $viewModel.editorTarget) { zone in
      DeviceLocalZoneEditorView(viewModel: viewModel.makeEditorViewModel(for: zone))
    }
    .sheet(isPresented: $viewModel.isSignUpCTAPresented) {
      DeviceLocalZoneSignUpCTAView(
        onCreateAccount: { viewModel.confirmSignUp() },
        onSignIn: { viewModel.confirmSignUp() },
        onDismiss: { viewModel.dismissSignUpCTA() }
      )
      .selfSizingSheet()
    }
  }

  // MARK: - Masthead

  private var mastheadRow: some View {
    MastheadView(title: "Zones")
      .padding(.horizontal, TCSpacing.medium)
      .padding(.top, TCSpacing.small)
      .padding(.bottom, TCSpacing.extraSmall)
      .listRowSeparator(.hidden)
      .listRowInsets(EdgeInsets())
      .listRowBackground(Color.tcBackground)
  }

  /// Load-bearing sign-up pitch (GH#888): this is the only remaining route to
  /// another area now the cap is one.
  ///
  /// Public Notice (GH#857/#896): styled as the anonymous-mode analogue of
  /// ``WatchZoneInlineUpsellCard`` — a brass small-caps eyebrow and a 1.5pt
  /// amber border, no fill. The "Create free account" `PrimaryButton` stays
  /// the only filled-amber container on this screen (amber-rationing rule).
  private var signUpPitchSection: some View {
    Section {
      VStack(alignment: .leading, spacing: TCSpacing.medium) {
        VStack(alignment: .leading, spacing: TCSpacing.small) {
          Text(Copy.eyebrow)
            .font(TCTypography.captionEmphasis)
            .textCase(.uppercase)
            .kerning(1.2)
            .foregroundStyle(Color.tcAmber)

          Text("Want more areas or alerts?")
            .font(TCTypography.headline)
            .foregroundStyle(Color.tcTextPrimary)
          Text(
            "Create a free account to save more areas and get notified when things change."
          )
          .font(TCTypography.body)
          .foregroundStyle(Color.tcTextSecondary)
        }

        HStack(spacing: TCSpacing.medium) {
          PrimaryButton("Create free account") { viewModel.requestSignUpFromPitch() }

          Button("Sign in") { viewModel.requestSignUpFromPitch() }
            .font(TCTypography.bodyEmphasis)
            .foregroundStyle(Color.tcTextSecondary)
        }
      }
      .padding(TCSpacing.medium)
      .background(Color.tcSurfaceElevated)
      .clipShape(RoundedRectangle(cornerRadius: TCCornerRadius.medium))
      .overlay(
        RoundedRectangle(cornerRadius: TCCornerRadius.medium)
          .strokeBorder(Color.tcAmber, lineWidth: 1.5)
      )
    }
    .cardRowInsets()
  }

  private var emptyState: some View {
    Section {
      VStack(spacing: TCSpacing.medium) {
        Image(systemName: "mappin.and.ellipse")
          .font(TCTypography.displayLarge)
          .foregroundStyle(Color.tcTextTertiary)
        Text("No Area Set Up")
          .font(TCTypography.headline)
        Text("Complete onboarding to set up your area.")
          .font(TCTypography.body)
          .foregroundStyle(Color.tcTextSecondary)
          .multilineTextAlignment(.center)
      }
      .padding(.vertical, TCSpacing.extraLarge)
    }
    .listRowBackground(Color.tcBackground)
  }
}

private struct DeviceLocalZoneRow: View {
  let zone: DeviceLocalZone
  let onAlertTap: () -> Void

  var body: some View {
    HStack(spacing: TCSpacing.medium) {
      ZoneMapPreview(centre: zone.centre, radiusMetres: zone.radiusMetres)
        .frame(width: 56, height: 56)
        .clipShape(RoundedRectangle(cornerRadius: TCCornerRadius.small))

      VStack(alignment: .leading, spacing: TCSpacing.extraSmall) {
        // Mono header strip: radius reads as the zone's metadata line, ahead
        // of its name (GH#857/#896) — mirrors the authed `WatchZoneRow`'s
        // own strip.
        Text(formatRadius(zone.radiusMetres))
          .font(TCTypography.mono)
          .foregroundStyle(Color.tcTextSecondary)
        Text(zone.name)
          .font(TCTypography.headline)
      }

      Spacer()

      // Any alert/notification affordance on a zone row is a sign-up CTA —
      // device-local zones never deliver alerts (GH#879 Phase 4).
      Button(action: onAlertTap) {
        Image(systemName: "bell.slash")
          .foregroundStyle(Color.tcTextTertiary)
      }
      .buttonStyle(.plain)
      .accessibilityLabel("Alerts require a free account")

      Image(systemName: "chevron.right")
        .font(TCTypography.caption)
        .foregroundStyle(Color.tcTextTertiary)
    }
    .padding(TCSpacing.medium)
    .noticeCardStyle()
  }
}
