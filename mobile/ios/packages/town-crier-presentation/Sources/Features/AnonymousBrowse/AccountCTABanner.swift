import SwiftUI

/// Persistent call-to-action pinned above the anonymous map's bottom safe
/// area (GH#868 Phase 3.4).
///
/// Copy is a deliberate product/legal decision — do not deviate, and never
/// say "instant": that's a paid-tier word (free accounts get the weekly
/// digest only; instant alerts are a paid, server-enforced entitlement
/// pitched on the wizard's own upsell step, not here).
public struct AccountCTABanner: View {
  public enum Copy {
    public static let headline = "Want to know when something changes here?"
    public static let subline = "Create a free account and we'll send you alerts for this area."
    public static let createAccount = "Create free account"
    public static let signIn = "Sign in"
  }

  private let onCreateAccount: () -> Void
  private let onSignIn: () -> Void

  public init(onCreateAccount: @escaping () -> Void, onSignIn: @escaping () -> Void) {
    self.onCreateAccount = onCreateAccount
    self.onSignIn = onSignIn
  }

  public var body: some View {
    VStack(alignment: .leading, spacing: TCSpacing.small) {
      Text(Copy.headline)
        .font(TCTypography.headline)
        .foregroundStyle(Color.tcTextPrimary)

      Text(Copy.subline)
        .font(TCTypography.body)
        .foregroundStyle(Color.tcTextSecondary)

      HStack(spacing: TCSpacing.medium) {
        PrimaryButton(Copy.createAccount, action: onCreateAccount)

        Button(Copy.signIn, action: onSignIn)
          .font(TCTypography.bodyEmphasis)
          .foregroundStyle(Color.tcTextSecondary)
      }
    }
    .padding(TCSpacing.medium)
    .background(Color.tcSurfaceElevated)
    .clipShape(RoundedRectangle(cornerRadius: TCCornerRadius.large))
    .padding(.horizontal, TCSpacing.medium)
    .padding(.bottom, TCSpacing.small)
  }
}
