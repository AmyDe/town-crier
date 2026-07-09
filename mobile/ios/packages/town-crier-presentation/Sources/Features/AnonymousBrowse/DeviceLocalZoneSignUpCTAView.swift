import SwiftUI

/// Sign-up CTA shown when an anonymous user hits the on-device zone cap
/// (``DeviceLocalZone/maxZoneCount``) or taps a zone row's alert affordance
/// (GH#879 Phase 4).
///
/// Copy is a deliberate product/legal decision, mirroring
/// ``AccountCTABanner``'s rationale: pitches the account, never promises
/// more on-device areas (the free tier is one server zone), and never says
/// "instant" — instant alerts are a paid, server-enforced entitlement.
///
/// Public Notice (GH#857/#896): this sheet is one of the two anonymous-mode
/// surfaces that gets the upsell-card treatment (the other is
/// ``DeviceLocalZoneListView``'s sign-up pitch section) — a brass small-caps
/// eyebrow plus a 1.5pt amber border. Amber rationing still holds: the
/// border + eyebrow are the sheet's only accent besides the "Create free
/// account" `PrimaryButton`, its one filled-amber container.
public struct DeviceLocalZoneSignUpCTAView: View {
  public enum Copy {
    public static let eyebrow = "Free Account"
    public static let headline = "Create a free account"
    public static let subline =
      "Get notified when things change in your areas, and keep them saved beyond this device."
    public static let createAccount = "Create free account"
    public static let signIn = "Sign in"
    public static let notNow = "Not now"
  }

  private let onCreateAccount: () -> Void
  private let onSignIn: () -> Void
  private let onDismiss: () -> Void

  public init(
    onCreateAccount: @escaping () -> Void,
    onSignIn: @escaping () -> Void,
    onDismiss: @escaping () -> Void
  ) {
    self.onCreateAccount = onCreateAccount
    self.onSignIn = onSignIn
    self.onDismiss = onDismiss
  }

  public var body: some View {
    VStack(spacing: TCSpacing.large) {
      Spacer()
        .frame(height: TCSpacing.medium)

      Image(systemName: "bell.badge")
        .font(TCTypography.displayLarge)
        .foregroundStyle(Color.tcAmber)

      // Brass small-caps eyebrow (upsell treatment, GH#857/#896).
      Text(Copy.eyebrow)
        .font(TCTypography.captionEmphasis)
        .textCase(.uppercase)
        .kerning(1.2)
        .foregroundStyle(Color.tcAmber)

      Text(Copy.headline)
        .font(TCTypography.displaySmall)
        .foregroundStyle(Color.tcTextPrimary)

      Text(Copy.subline)
        .font(TCTypography.body)
        .foregroundStyle(Color.tcTextSecondary)
        .multilineTextAlignment(.center)
        .padding(.horizontal, TCSpacing.medium)

      Spacer()
        .frame(height: TCSpacing.small)

      PrimaryButton(Copy.createAccount, action: onCreateAccount)
        .padding(.horizontal, TCSpacing.medium)

      Button(action: onSignIn) {
        Text(Copy.signIn)
          .font(TCTypography.bodyEmphasis)
          .foregroundStyle(Color.tcTextSecondary)
      }
      .frame(minHeight: 44)

      Button(action: onDismiss) {
        Text(Copy.notNow)
          .font(TCTypography.body)
          .foregroundStyle(Color.tcTextTertiary)
      }
      .frame(minHeight: 44)

      Spacer()
        .frame(height: TCSpacing.medium)
    }
    .padding(.horizontal, TCSpacing.medium)
    .background(Color.tcSurfaceElevated)
    .clipShape(RoundedRectangle(cornerRadius: TCCornerRadius.large))
    .overlay(
      // Upsell treatment border (GH#857/#896) — no fill, matching the
      // design language's "outline carries the accent" rule.
      RoundedRectangle(cornerRadius: TCCornerRadius.large)
        .strokeBorder(Color.tcAmber, lineWidth: 1.5)
    )
  }

  // MARK: - Test Helpers

  func simulateCreateAccountTap() {
    onCreateAccount()
  }

  func simulateSignInTap() {
    onSignIn()
  }

  func simulateDismissTap() {
    onDismiss()
  }
}
