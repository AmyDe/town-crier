import SwiftUI

/// Displays a legal document (privacy policy or terms of service) with
/// a scrollable list of titled sections.
public struct LegalDocumentView: View {
    private let viewModel: LegalDocumentViewModel

    public init(viewModel: LegalDocumentViewModel) {
        self.viewModel = viewModel
    }

    public var body: some View {
        ScrollView {
            VStack(alignment: .leading, spacing: TCSpacing.large) {
                Text("Last updated: \(viewModel.lastUpdated)")
                    .font(TCTypography.caption)
                    .foregroundStyle(Color.tcTextSecondary)

                ForEach(Array(viewModel.sections.enumerated()), id: \.offset) { _, section in
                    SectionView(section: section)
                }
            }
            .padding(TCSpacing.medium)
        }
        .frame(maxWidth: .infinity, maxHeight: .infinity)
        .background(Color.tcBackground)
        .navigationTitle(viewModel.title)
    }
}

// MARK: - Section View

private struct SectionView: View {
    let section: LegalDocumentSection

    var body: some View {
        VStack(alignment: .leading, spacing: TCSpacing.small) {
            Text(section.heading)
                .font(TCTypography.headline)
                .foregroundStyle(Color.tcTextPrimary)

            Text(section.body)
                .font(TCTypography.body)
                .foregroundStyle(Color.tcTextSecondary)
        }
    }
}
