import SwiftUI

/// First screen a fresh install ever sees when there is no authenticated
/// session and no persisted anonymous browse state (GH#868 Phase 3). "Get
/// started" routes to postcode entry with no account and no Auth0 call;
/// "I already have an account" routes into the existing Auth0 login flow.
public struct WelcomeView: View {
  @StateObject private var viewModel: WelcomeViewModel

  public init(viewModel: WelcomeViewModel) {
    _viewModel = StateObject(wrappedValue: viewModel)
  }

  public var body: some View {
    VStack(spacing: TCSpacing.large) {
      Spacer()

      Image(systemName: "bell.badge")
        .font(.system(.largeTitle))
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
  }
}
