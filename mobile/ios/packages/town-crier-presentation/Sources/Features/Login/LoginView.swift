import SwiftUI
import TownCrierDomain

/// Login screen presenting Auth0 authentication options.
public struct LoginView: View {
  @StateObject private var viewModel: LoginViewModel

  public init(viewModel: LoginViewModel) {
    _viewModel = StateObject(wrappedValue: viewModel)
  }

  public var body: some View {
    VStack(spacing: TCSpacing.extraLarge) {
      Spacer()

      brandingSection

      Spacer()

      loginButton

      if let error = viewModel.error {
        errorMessage(error)
      }

      Spacer()
    }
    .padding(.horizontal, TCSpacing.large)
    .frame(maxWidth: .infinity, maxHeight: .infinity)
    .background(Color.tcBackground)
    .task {
      await viewModel.checkExistingSession()
    }
  }

  private var brandingSection: some View {
    VStack(spacing: TCSpacing.medium) {
      Image(systemName: "bell.badge")
        .font(.system(.largeTitle))
        .foregroundStyle(Color.tcAmber)
        .imageScale(.large)

      Text("Town Crier")
        .font(TCTypography.displayLarge)
        .foregroundStyle(Color.tcTextPrimary)

      Text("Planning applications near you")
        .font(TCTypography.body)
        .foregroundStyle(Color.tcTextSecondary)
    }
  }

  private var loginButton: some View {
    PrimaryButton {
      Task {
        await viewModel.login()
      }
    } label: {
      if viewModel.isLoading {
        ProgressView()
          .tint(Color.tcTextOnAccent)
      } else {
        Text("Sign in")
      }
    }
    .disabled(viewModel.isLoading)
  }

  private func errorMessage(_ error: DomainError) -> some View {
    Text(errorDescription(error))
      .font(TCTypography.caption)
      .foregroundStyle(Color.tcStatusRejected)
      .multilineTextAlignment(.center)
  }

  private func errorDescription(_ error: DomainError) -> String {
    switch error {
    case .authenticationFailed:
      return "Sign in failed. Please try again."
    case .sessionExpired:
      return "Your session has expired. Please sign in again."
    default:
      return "Something went wrong. Please try again."
    }
  }
}
