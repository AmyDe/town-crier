import SwiftUI
import TownCrierDomain

/// Redeem offer code screen reached from Settings.
///
/// A focused form with a monospaced, auto-uppercased text field for the
/// `XXXX-XXXX-XXXX` code, a primary CTA that shows a spinner while the request
/// is in flight, and inline error messaging when the ViewModel reports a
/// mapped `OfferCodeError`. On success, a native alert confirms the new tier
/// and expiry date before the Coordinator dismisses the screen via the
/// ViewModel's `onRedeemed` callback.
public struct RedeemOfferCodeView: View {
  @StateObject private var viewModel: RedeemOfferCodeViewModel
  @Environment(\.dismiss) private var dismiss
  @State private var presentedRedemption: OfferCodeRedemption?

  public init(viewModel: RedeemOfferCodeViewModel) {
    _viewModel = StateObject(wrappedValue: viewModel)
  }

  public var body: some View {
    Form {
      codeSection
      if let errorMessage = viewModel.errorMessage {
        errorSection(errorMessage)
      }
      redeemSection
    }
    .background(Color.tcBackground)
    .scrollContentBackground(.hidden)
    .navigationTitle("Redeem Offer Code")
    #if os(iOS)
      .navigationBarTitleDisplayMode(.inline)
    #endif
    .onChange(of: viewModel.redemption) { _, newValue in
      presentedRedemption = newValue
    }
    .alert(
      "Subscription activated",
      isPresented: Binding(
        get: { presentedRedemption != nil },
        set: { if !$0 { presentedRedemption = nil } }
      ),
      presenting: presentedRedemption
    ) { _ in
      Button("Done") { dismiss() }
    } message: { redemption in
      Text(Self.successMessage(for: redemption))
    }
  }

  // MARK: - Sections

  private var codeSection: some View {
    Section {
      TextField("XXXX-XXXX-XXXX", text: $viewModel.code)
        .font(.system(.body, design: .monospaced))
        .autocorrectionDisabled()
        #if os(iOS)
          .textInputAutocapitalization(.characters)
        #endif
        .foregroundStyle(Color.tcTextPrimary)
    } header: {
      Text("Offer Code")
        .font(TCTypography.captionEmphasis)
    } footer: {
      Text("Enter the 12-character code you received. Dashes and spaces are optional.")
        .font(TCTypography.caption)
        .foregroundStyle(Color.tcTextSecondary)
    }
  }

  private func errorSection(_ message: String) -> some View {
    Section {
      Label {
        Text(message)
          .font(TCTypography.body)
          .foregroundStyle(Color.tcStatusRejected)
      } icon: {
        Image(systemName: "exclamationmark.triangle.fill")
          .foregroundStyle(Color.tcStatusRejected)
      }
    }
  }

  private var redeemSection: some View {
    Section {
      PrimaryButton {
        Task { await viewModel.redeem() }
      } label: {
        if viewModel.isLoading {
          ProgressView()
            .tint(Color.tcTextOnAccent)
        } else {
          Text("Redeem")
        }
      }
      .disabled(viewModel.isLoading || viewModel.code.isEmpty)
      .listRowInsets(
        EdgeInsets(
          top: TCSpacing.small,
          leading: TCSpacing.medium,
          bottom: TCSpacing.small,
          trailing: TCSpacing.medium
        )
      )
      .listRowBackground(Color.clear)
    }
  }

  // MARK: - Presentation helpers

  private static func successMessage(for redemption: OfferCodeRedemption) -> String {
    let tier = redemption.tier.rawValue.capitalized
    let formatter = DateFormatter()
    formatter.dateStyle = .medium
    formatter.timeStyle = .none
    let expires = formatter.string(from: redemption.expiresAt)
    return "You're on \(tier) until \(expires). Enjoy!"
  }
}
