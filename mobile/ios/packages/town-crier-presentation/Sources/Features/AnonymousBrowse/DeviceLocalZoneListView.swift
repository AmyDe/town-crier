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
  @StateObject private var viewModel: DeviceLocalZoneListViewModel

  public init(viewModel: DeviceLocalZoneListViewModel) {
    _viewModel = StateObject(wrappedValue: viewModel)
  }

  public var body: some View {
    List {
      if let zone = viewModel.zones.first {
        DeviceLocalZoneRow(zone: zone) { viewModel.requestAlertsSignUp() }
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

  /// Load-bearing sign-up pitch (GH#888): this is the only remaining route to
  /// another area now the cap is one.
  private var signUpPitchSection: some View {
    Section {
      VStack(alignment: .leading, spacing: TCSpacing.small) {
        Text("Want more areas or alerts?")
          .font(TCTypography.headline)
          .foregroundStyle(Color.tcTextPrimary)
        Text(
          "Create a free account to save more areas and get notified when things change."
        )
        .font(TCTypography.body)
        .foregroundStyle(Color.tcTextSecondary)

        HStack(spacing: TCSpacing.medium) {
          PrimaryButton("Create free account") { viewModel.requestSignUpFromPitch() }

          Button("Sign in") { viewModel.requestSignUpFromPitch() }
            .font(TCTypography.bodyEmphasis)
            .foregroundStyle(Color.tcTextSecondary)
        }
      }
      .padding(.vertical, TCSpacing.small)
    }
    .listRowBackground(Color.tcSurface)
  }

  private var emptyState: some View {
    Section {
      VStack(spacing: TCSpacing.medium) {
        Image(systemName: "mappin.and.ellipse")
          .font(.system(.largeTitle))
          .foregroundStyle(Color.tcTextTertiary)
        Text("No Area Set Up")
          .font(.system(.headline).weight(.semibold))
        Text("Complete onboarding to set up your area.")
          .font(.system(.body))
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
        Text(zone.name)
          .font(.system(.headline).weight(.semibold))
        Text(formatRadius(zone.radiusMetres))
          .font(.system(.caption))
          .foregroundStyle(Color.tcTextSecondary)
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
        .font(.system(.caption))
        .foregroundStyle(Color.tcTextTertiary)
    }
    .padding(.vertical, TCSpacing.extraSmall)
    .listRowBackground(Color.tcSurface)
  }
}
