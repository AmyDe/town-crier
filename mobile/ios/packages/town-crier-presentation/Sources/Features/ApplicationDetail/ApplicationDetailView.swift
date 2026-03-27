import SwiftUI
import TownCrierDomain

#if os(iOS)
import SafariServices
#endif

/// Full detail view for a planning application.
public struct ApplicationDetailView: View {
    @StateObject private var viewModel: ApplicationDetailViewModel
    @State private var showingSafari = false

    public init(viewModel: ApplicationDetailViewModel) {
        _viewModel = StateObject(wrappedValue: viewModel)
    }

    public var body: some View {
        ScrollView {
            VStack(alignment: .leading, spacing: TCSpacing.large) {
                statusBadge

                Text(viewModel.description)
                    .font(TCTypography.displaySmall)
                    .foregroundStyle(Color.tcTextPrimary)

                detailCard

                if viewModel.hasTimeline {
                    VStack(alignment: .leading, spacing: TCSpacing.small) {
                        Text("Status Timeline")
                            .font(TCTypography.headline)
                            .foregroundStyle(Color.tcTextPrimary)

                        StatusTimelineView(items: viewModel.timelineItems)
                    }
                }

                if viewModel.hasPortalUrl {
                    portalButton
                }
            }
            .padding(TCSpacing.medium)
        }
        .frame(maxWidth: .infinity, alignment: .leading)
        .background(Color.tcBackground)
        .navigationTitle("Application")
        #if os(iOS)
        .navigationBarTitleDisplayMode(.inline)
        .sheet(isPresented: $showingSafari) {
            if let url = viewModel.portalUrl {
                SafariView(url: url)
            }
        }
        #endif
    }

    // MARK: - Status Badge

    private var statusBadge: some View {
        HStack(spacing: TCSpacing.extraSmall) {
            Image(systemName: viewModel.statusIcon)
            Text(viewModel.statusLabel)
        }
        .font(TCTypography.captionEmphasis)
        .foregroundStyle(statusColor)
        .padding(.horizontal, TCSpacing.small)
        .padding(.vertical, TCSpacing.extraSmall)
        .background(statusColor.opacity(0.15))
        .clipShape(Capsule())
    }

    // MARK: - Detail Card

    private var detailCard: some View {
        VStack(alignment: .leading, spacing: TCSpacing.medium) {
            detailRow(icon: "mappin.and.ellipse", label: "Address", value: viewModel.address)
            Divider().foregroundStyle(Color.tcBorder)
            detailRow(icon: "doc.text", label: "Reference", value: viewModel.reference)
            Divider().foregroundStyle(Color.tcBorder)
            detailRow(icon: "building.2", label: "Authority", value: viewModel.authorityName)
            Divider().foregroundStyle(Color.tcBorder)
            detailRow(icon: "calendar", label: "Received", value: viewModel.receivedDateFormatted)
        }
        .padding(TCSpacing.medium)
        .background(Color.tcSurface)
        .clipShape(RoundedRectangle(cornerRadius: TCCornerRadius.medium))
    }

    private func detailRow(icon: String, label: String, value: String) -> some View {
        HStack(alignment: .top, spacing: TCSpacing.small) {
            Image(systemName: icon)
                .font(TCTypography.body)
                .foregroundStyle(Color.tcTextSecondary)
                .frame(width: 24, alignment: .center)

            VStack(alignment: .leading, spacing: TCSpacing.extraSmall) {
                Text(label)
                    .font(TCTypography.caption)
                    .foregroundStyle(Color.tcTextTertiary)
                Text(value)
                    .font(TCTypography.body)
                    .foregroundStyle(Color.tcTextPrimary)
            }
        }
    }

    // MARK: - Portal Button

    private var portalButton: some View {
        PrimaryButton {
            showingSafari = true
        } label: {
            HStack {
                Image(systemName: "safari")
                Text("View on Council Portal")
            }
        }
    }

    // MARK: - Status Color

    private var statusColor: Color {
        viewModel.status.displayColor
    }
}

#if os(iOS)
/// Wraps SFSafariViewController for use in SwiftUI.
struct SafariView: UIViewControllerRepresentable {
    let url: URL

    func makeUIViewController(context: Context) -> SFSafariViewController {
        SFSafariViewController(url: url)
    }

    func updateUIViewController(_ uiViewController: SFSafariViewController, context: Context) {}
}
#endif
