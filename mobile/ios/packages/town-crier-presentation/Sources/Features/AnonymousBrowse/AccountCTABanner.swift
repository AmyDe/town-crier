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

extension View {
  /// Pins ``AccountCTABanner`` above this content's bottom safe area
  /// (GH#879 Phase 3). Apply INSIDE each tab's content in
  /// `AnonymousMainTabView` — never on the enclosing `TabView` itself.
  ///
  /// `.safeAreaInset(edge: .bottom)` applied directly to a `TabView` insets
  /// against the *window's* bottom edge rather than the safe area above the
  /// tab bar, drawing the banner over the tab bar and swallowing taps on the
  /// other tabs — a real, reproducible SwiftUI layout defect confirmed via
  /// live simulator verification, not a simulator artifact. Applying the
  /// same modifier to a tab's own content instead works correctly, because
  /// that content's bottom edge is already the boundary just above the tab
  /// bar — mirrors the pre-Phase-3 `AnonymousMapView`, which hosted this
  /// exact banner inside its own content and stacked correctly.
  ///
  /// Using `.safeAreaInset` (rather than a manual `ZStack` overlay) also
  /// means scrollable content — the Applications list — automatically
  /// reserves space for the banner, so its last row is never permanently
  /// hidden behind it.
  func accountCTABanner(
    onCreateAccount: @escaping () -> Void,
    onSignIn: @escaping () -> Void
  ) -> some View {
    safeAreaInset(edge: .bottom) {
      AccountCTABanner(onCreateAccount: onCreateAccount, onSignIn: onSignIn)
    }
  }
}
