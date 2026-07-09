import SwiftUI

/// First screen a fresh install ever sees when there is no authenticated
/// session and no persisted anonymous browse state (GH#868 Phase 3). "Get
/// started" routes to postcode entry with no account and no Auth0 call;
/// "I already have an account" routes into the existing Auth0 login flow.
///
/// Also carries a compact top-trailing appearance control (GH#878): an
/// anonymous user has no way into Settings, so this is the only place they
/// can switch off the system theme before creating an account.
public struct WelcomeView: View {
  @StateObject private var viewModel: WelcomeViewModel

  public init(viewModel: WelcomeViewModel) {
    _viewModel = StateObject(wrappedValue: viewModel)
  }

  public var body: some View {
    VStack(spacing: TCSpacing.large) {
      Spacer()

      Image(systemName: "bell.badge")
        .font(TCTypography.displayLarge)
        .foregroundStyle(Color.tcAmber)

      Text("Town Crier")
        .font(TCTypography.displayLarge)
        .foregroundStyle(Color.tcTextPrimary)
        .multilineTextAlignment(.center)

      Text("See planning applications happening near you, before you create an account.")
        .font(TCTypography.body)
        .foregroundStyle(Color.tcTextSecondary)
        .multilineTextAlignment(.center)
        .padding(.horizontal, TCSpacing.extraLarge)

      Spacer()

      PrimaryButton("Get started", action: viewModel.getStarted)
        .padding(.horizontal, TCSpacing.extraLarge)

      Button("I already have an account", action: viewModel.signIn)
        .font(TCTypography.body)
        .foregroundStyle(Color.tcTextSecondary)
        .padding(.bottom, TCSpacing.large)
    }
    .frame(maxWidth: .infinity, maxHeight: .infinity)
    .background(Color.tcBackground)
    .overlay(alignment: .topTrailing) {
      appearanceMenu
        .padding(.top, TCSpacing.small)
        .padding(.trailing, TCSpacing.medium)
    }
  }

  /// Top-trailing icon button opening a `Menu` of all four `AppearanceMode`
  /// cases — same set and display names as the Settings picker. A `Picker`
  /// nested inside the `Menu` gets SwiftUI's built-in checkmark-on-selected
  /// rendering for free, so the current mode is indicated without hand-rolled
  /// per-row styling.
  private var appearanceMenu: some View {
    Menu {
      Picker(selection: appearanceModeBinding) {
        ForEach(AppearanceMode.allCases, id: \.self) { mode in
          Text(mode.displayName).tag(mode)
        }
      } label: {
        EmptyView()
      }
    } label: {
      Image(systemName: "circle.lefthalf.filled")
        .font(TCTypography.body)
        .foregroundStyle(Color.tcTextSecondary)
        // Apple HIG minimum tap target, matching the Buttons component spec.
        .frame(width: 44, height: 44)
        .contentShape(Rectangle())
    }
    .accessibilityLabel("Appearance")
  }

  private var appearanceModeBinding: Binding<AppearanceMode> {
    Binding(
      get: { viewModel.appearanceMode },
      set: { viewModel.selectAppearanceMode($0) }
    )
  }
}
