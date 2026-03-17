import SwiftUI
import TownCrierDomain

/// Centralised settings screen: account, notifications, subscription, attribution, and account management.
public struct SettingsView: View {
    @StateObject private var viewModel: SettingsViewModel
    var onNotificationPreferences: (() -> Void)?
    var onManageSubscription: (() -> Void)?
    var onPrivacyPolicy: (() -> Void)?
    var onTermsOfService: (() -> Void)?

    public init(viewModel: SettingsViewModel) {
        _viewModel = StateObject(wrappedValue: viewModel)
    }

    public var body: some View {
        List {
            accountSection
            notificationSection
            subscriptionSection
            attributionSection
            legalSection
            dangerZoneSection
            appInfoSection
        }
        .background(Color.tcBackground)
        .scrollContentBackground(.hidden)
        .navigationTitle("Settings")
        .task { await viewModel.load() }
        .alert("Delete Account", isPresented: $viewModel.isShowingDeleteConfirmation) {
            Button("Delete", role: .destructive) {
                Task { await viewModel.confirmDeleteAccount() }
            }
            Button("Cancel", role: .cancel) {
                viewModel.cancelDeletion()
            }
        } message: {
            Text("This will permanently delete your account and all associated data. This action cannot be undone.")
        }
    }

    // MARK: - Account

    private var accountSection: some View {
        Section {
            if let email = viewModel.userEmail {
                HStack {
                    Text("Email")
                        .font(TCTypography.body)
                        .foregroundStyle(Color.tcTextPrimary)
                    Spacer()
                    Text(email)
                        .font(TCTypography.body)
                        .foregroundStyle(Color.tcTextSecondary)
                }

                if let name = viewModel.userName {
                    HStack {
                        Text("Name")
                            .font(TCTypography.body)
                            .foregroundStyle(Color.tcTextPrimary)
                        Spacer()
                        Text(name)
                            .font(TCTypography.body)
                            .foregroundStyle(Color.tcTextSecondary)
                    }
                }

                if let method = viewModel.authMethod {
                    HStack {
                        Text("Sign-in Method")
                            .font(TCTypography.body)
                            .foregroundStyle(Color.tcTextPrimary)
                        Spacer()
                        Text(method.displayName)
                            .font(TCTypography.body)
                            .foregroundStyle(Color.tcTextSecondary)
                    }
                }
            }
        } header: {
            Text("Account")
                .font(TCTypography.captionEmphasis)
        }
    }

    // MARK: - Notifications

    private var notificationSection: some View {
        Section {
            Button {
                onNotificationPreferences?()
            } label: {
                HStack {
                    Label("Notification Preferences", systemImage: "bell")
                        .font(TCTypography.body)
                        .foregroundStyle(Color.tcTextPrimary)
                    Spacer()
                    Image(systemName: "chevron.right")
                        .font(.system(.caption))
                        .foregroundStyle(Color.tcTextTertiary)
                }
            }
        } header: {
            Text("Notifications")
                .font(TCTypography.captionEmphasis)
        }
    }

    // MARK: - Subscription

    private var subscriptionSection: some View {
        Section {
            HStack {
                Text("Current Plan")
                    .font(TCTypography.body)
                    .foregroundStyle(Color.tcTextPrimary)
                Spacer()
                HStack(spacing: TCSpacing.extraSmall) {
                    Text(viewModel.subscriptionTier.rawValue.capitalized)
                        .font(TCTypography.bodyEmphasis)
                        .foregroundStyle(Color.tcTextPrimary)
                    if viewModel.isTrialPeriod {
                        Text("Trial")
                            .font(TCTypography.captionEmphasis)
                            .foregroundStyle(Color.tcStatusPending)
                    }
                }
            }

            Button {
                onManageSubscription?()
            } label: {
                HStack {
                    Label("Manage Subscription", systemImage: "creditcard")
                        .font(TCTypography.body)
                        .foregroundStyle(Color.tcTextPrimary)
                    Spacer()
                    Image(systemName: "chevron.right")
                        .font(.system(.caption))
                        .foregroundStyle(Color.tcTextTertiary)
                }
            }
        } header: {
            Text("Subscription")
                .font(TCTypography.captionEmphasis)
        }
    }

    // MARK: - Attribution

    private var attributionSection: some View {
        Section {
            ForEach(viewModel.attributionItems, id: \.name) { item in
                VStack(alignment: .leading, spacing: TCSpacing.extraSmall) {
                    Text(item.name)
                        .font(TCTypography.bodyEmphasis)
                        .foregroundStyle(Color.tcTextPrimary)
                    Text(item.detail)
                        .font(TCTypography.caption)
                        .foregroundStyle(Color.tcTextSecondary)
                }
            }
        } header: {
            Text("Data Attribution")
                .font(TCTypography.captionEmphasis)
        }
    }

    // MARK: - Legal

    private var legalSection: some View {
        Section {
            Button {
                onPrivacyPolicy?()
            } label: {
                HStack {
                    Label("Privacy Policy", systemImage: "hand.raised")
                        .font(TCTypography.body)
                        .foregroundStyle(Color.tcTextPrimary)
                    Spacer()
                    Image(systemName: "chevron.right")
                        .font(.system(.caption))
                        .foregroundStyle(Color.tcTextTertiary)
                }
            }

            Button {
                onTermsOfService?()
            } label: {
                HStack {
                    Label("Terms of Service", systemImage: "doc.text")
                        .font(TCTypography.body)
                        .foregroundStyle(Color.tcTextPrimary)
                    Spacer()
                    Image(systemName: "chevron.right")
                        .font(.system(.caption))
                        .foregroundStyle(Color.tcTextTertiary)
                }
            }
        } header: {
            Text("Legal")
                .font(TCTypography.captionEmphasis)
        }
    }

    // MARK: - Danger Zone

    private var dangerZoneSection: some View {
        Section {
            Button {
                Task { await viewModel.logout() }
            } label: {
                Label("Sign Out", systemImage: "rectangle.portrait.and.arrow.right")
                    .font(TCTypography.body)
                    .foregroundStyle(Color.tcTextPrimary)
            }

            Button(role: .destructive) {
                viewModel.requestAccountDeletion()
            } label: {
                Label("Delete Account", systemImage: "trash")
                    .font(TCTypography.body)
            }
        } header: {
            Text("Account")
                .font(TCTypography.captionEmphasis)
        }
    }

    // MARK: - App Info

    private var appInfoSection: some View {
        Section {
            HStack {
                Text("Version")
                    .font(TCTypography.body)
                    .foregroundStyle(Color.tcTextPrimary)
                Spacer()
                Text(viewModel.appVersion)
                    .font(TCTypography.caption)
                    .foregroundStyle(Color.tcTextSecondary)
            }
        }
    }
}
