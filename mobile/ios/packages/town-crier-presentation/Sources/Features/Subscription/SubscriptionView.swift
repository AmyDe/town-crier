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
            }
            .padding(.horizontal, TCSpacing.medium)
            .padding(.top, TCSpacing.extraLarge)
            .padding(.bottom, TCSpacing.large)
        }
        .background(Color.tcBackground)
        .task { await viewModel.loadProducts() }
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

    private func trialBadge(days: Int) -> some View {
        Text("\(days)-day free trial")
            .font(TCTypography.captionEmphasis)
            .foregroundStyle(Color.tcStatusApproved)
            .padding(.horizontal, TCSpacing.small)
            .padding(.vertical, TCSpacing.extraSmall)
            .background(Color.tcStatusApproved.opacity(0.15))
            .clipShape(Capsule())
    }

    private func purchaseButton(for product: SubscriptionProduct) -> some View {
        Button {
            Task { await viewModel.purchase(productId: product.id) }
        } label: {
            Group {
                if viewModel.isPurchasing {
                    ProgressView()
                        .tint(Color.tcTextOnAccent)
                } else {
                    Text(product.hasFreeTrial ? "Start Free Trial" : "Subscribe")
                        .font(TCTypography.bodyEmphasis)
                }
            }
            .frame(maxWidth: .infinity)
            .frame(minHeight: 44)
        }
        .foregroundStyle(Color.tcTextOnAccent)
        .background(Color.tcAmber)
        .clipShape(RoundedRectangle(cornerRadius: TCCornerRadius.medium))
        .disabled(viewModel.isPurchasing)
    }

    private var currentPlanLabel: some View {
        HStack {
            Image(systemName: "checkmark.circle.fill")
                .foregroundStyle(Color.tcStatusApproved)
            Text("Current plan")
                .font(TCTypography.bodyEmphasis)
                .foregroundStyle(Color.tcStatusApproved)
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

            Button {
                Task { await viewModel.loadProducts() }
            } label: {
                Text("Try Again")
                    .font(TCTypography.bodyEmphasis)
                    .frame(maxWidth: .infinity)
                    .frame(minHeight: 44)
            }
            .foregroundStyle(Color.tcTextOnAccent)
            .background(Color.tcAmber)
            .clipShape(RoundedRectangle(cornerRadius: TCCornerRadius.medium))
        }
        .padding(.top, TCSpacing.large)
    }
}
