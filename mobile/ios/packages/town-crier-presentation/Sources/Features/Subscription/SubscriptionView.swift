import SwiftUI
import TownCrierDomain

/// Subscription paywall displaying products, purchase buttons, and App Store disclosures.
public struct SubscriptionView: View {
  @StateObject private var viewModel: SubscriptionViewModel

  public init(viewModel: SubscriptionViewModel) {
    _viewModel = StateObject(wrappedValue: viewModel)
  }

  public var body: some View {
    ScrollView {
      VStack(spacing: TCSpacing.large) {
        headerSection

        if viewModel.isLoading {
          ProgressView()
            .padding(.top, TCSpacing.extraLarge)
        } else if let error = viewModel.error {
          errorSection(error)
        } else {
          productsSection
          restoreSection
        }

        // Always visible — App Store Guideline 3.1.2(c) requires functional
        // Privacy Policy and Terms of Use links on the paywall itself, even
        // when products fail to load.
        legalLinksFooter
      }
      .padding(.horizontal, TCSpacing.medium)
      .padding(.top, TCSpacing.extraLarge)
      .padding(.bottom, TCSpacing.large)
    }
    .background(Color.tcBackground)
    .task { await viewModel.loadProducts() }
    .sheet(item: $viewModel.presentedLegalDocument) { documentType in
      NavigationStack {
        LegalDocumentView(viewModel: LegalDocumentViewModel(documentType: documentType))
      }
    }
  }

  // MARK: - Header

  private var headerSection: some View {
    VStack(spacing: TCSpacing.small) {
      Image(systemName: "bell.badge.fill")
        .font(.system(.largeTitle))
        .foregroundStyle(Color.tcAmber)

      Text("Upgrade Town Crier")
        .font(TCTypography.displayLarge)
        .foregroundStyle(Color.tcTextPrimary)

      Text("Get more from your planning alerts")
        .font(TCTypography.body)
        .foregroundStyle(Color.tcTextSecondary)
    }
    .multilineTextAlignment(.center)
  }

  // MARK: - Products

  private var productsSection: some View {
    VStack(spacing: TCSpacing.medium) {
      ForEach(viewModel.products, id: \.id) { product in
        productCard(product)
      }
    }
  }

  private func productCard(_ product: SubscriptionProduct) -> some View {
    VStack(alignment: .leading, spacing: TCSpacing.small) {
      HStack {
        VStack(alignment: .leading, spacing: TCSpacing.extraSmall) {
          Text(product.displayName)
            .font(TCTypography.headline)
            .foregroundStyle(Color.tcTextPrimary)

          Text("\(product.displayPrice)/month")
            .font(TCTypography.body)
            .foregroundStyle(Color.tcTextSecondary)
        }

        Spacer()

        if product.hasFreeTrial {
          trialBadge(days: product.trialDays)
        }
      }

      featureList(for: product)
        .padding(.top, TCSpacing.extraSmall)
        .padding(.bottom, TCSpacing.small)

      if isCurrentTier(product) {
        currentPlanLabel
      } else {
        purchaseButton(for: product)
      }

      Text(viewModel.subscriptionDisclosure(for: product))
        .font(TCTypography.caption)
        .foregroundStyle(Color.tcTextTertiary)
    }
    .padding(TCSpacing.medium)
    .background(Color.tcSurface)
    .clipShape(RoundedRectangle(cornerRadius: TCCornerRadius.medium))
  }

  private func featureList(for product: SubscriptionProduct) -> some View {
    VStack(alignment: .leading, spacing: TCSpacing.small) {
      ForEach(product.tier.featureHighlights, id: \.self) { feature in
        Label {
          Text(feature)
            .font(TCTypography.body)
            .foregroundStyle(Color.tcTextPrimary)
            .fixedSize(horizontal: false, vertical: true)
        } icon: {
          Image(systemName: "checkmark.circle.fill")
            .font(TCTypography.body)
            .foregroundStyle(Color.tcAmber)
        }
      }
    }
    .frame(maxWidth: .infinity, alignment: .leading)
  }

  private func trialBadge(days: Int) -> some View {
    Text("\(days)-day free trial")
      .font(TCTypography.captionEmphasis)
      .foregroundStyle(Color.tcStatusPermitted)
      .padding(.horizontal, TCSpacing.small)
      .padding(.vertical, TCSpacing.extraSmall)
      .background(Color.tcStatusPermitted.opacity(0.15))
      .clipShape(Capsule())
  }

  private func purchaseButton(for product: SubscriptionProduct) -> some View {
    PrimaryButton {
      Task { await viewModel.purchase(productId: product.id) }
    } label: {
      if viewModel.isPurchasing {
        ProgressView()
          .tint(Color.tcTextOnAccent)
      } else {
        Text(product.hasFreeTrial ? "Start Free Trial" : "Subscribe")
      }
    }
    .disabled(viewModel.isPurchasing)
  }

  private var currentPlanLabel: some View {
    HStack {
      Image(systemName: "checkmark.circle.fill")
        .foregroundStyle(Color.tcStatusPermitted)
      Text("Current plan")
        .font(TCTypography.bodyEmphasis)
        .foregroundStyle(Color.tcStatusPermitted)
    }
    .frame(maxWidth: .infinity)
    .frame(minHeight: 44)
  }

  private func isCurrentTier(_ product: SubscriptionProduct) -> Bool {
    viewModel.currentEntitlement?.tier == product.tier
  }

  // MARK: - Restore

  private var restoreSection: some View {
    Button {
      Task { await viewModel.restorePurchases() }
    } label: {
      if viewModel.isRestoring {
        ProgressView()
      } else {
        Text("Restore Purchases")
          .font(TCTypography.body)
          .foregroundStyle(Color.tcAmber)
      }
    }
    .disabled(viewModel.isRestoring)
    .padding(.top, TCSpacing.small)
  }

  // MARK: - Legal

  private var legalLinksFooter: some View {
    HStack(spacing: TCSpacing.extraSmall) {
      legalLink("Privacy Policy") { viewModel.showLegalDocument(.privacyPolicy) }

      Text("·")
        .font(TCTypography.caption)
        .foregroundStyle(Color.tcTextTertiary)

      legalLink("Terms of Use") { viewModel.showLegalDocument(.termsOfService) }
    }
    .frame(minHeight: 44)
  }

  private func legalLink(_ title: String, action: @escaping () -> Void) -> some View {
    Button(action: action) {
      Text(title)
        .font(TCTypography.caption)
        .foregroundStyle(Color.tcAmber)
    }
  }

  // MARK: - Error

  private func errorSection(_ error: DomainError) -> some View {
    VStack(spacing: TCSpacing.medium) {
      Image(systemName: "exclamationmark.triangle")
        .font(.system(.largeTitle))
        .foregroundStyle(Color.tcTextTertiary)

      Text("Something went wrong")
        .font(TCTypography.headline)
        .foregroundStyle(Color.tcTextPrimary)

      Text("Unable to load subscription options. Please try again.")
        .font(TCTypography.body)
        .foregroundStyle(Color.tcTextSecondary)
        .multilineTextAlignment(.center)

      PrimaryButton("Try Again") {
        Task { await viewModel.loadProducts() }
      }
    }
    .padding(.top, TCSpacing.large)
  }
}
