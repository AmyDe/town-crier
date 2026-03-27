import SwiftUI
import TownCrierDomain

/// Filterable list of planning applications within a watch zone.
public struct ApplicationListView: View {
    @StateObject private var viewModel: ApplicationListViewModel

    public init(viewModel: ApplicationListViewModel) {
        _viewModel = StateObject(wrappedValue: viewModel)
    }

    public var body: some View {
        ZStack {
            Color.tcBackground.ignoresSafeArea()

            if viewModel.isLoading && viewModel.filteredApplications.isEmpty {
                ListSkeletonView()
            } else if let error = viewModel.error {
                ErrorStateView(error: error) {
                    await viewModel.loadApplications()
                }
            } else if viewModel.isEmpty {
                EmptyStateView(
                    icon: "doc.text.magnifyingglass",
                    title: "No Applications",
                    description: "No planning applications found in your watch zone yet."
                )
            } else {
                applicationList
            }
        }
        .navigationTitle("Applications")
        #if os(iOS)
        .navigationBarTitleDisplayMode(.large)
        #endif
        .task {
            await viewModel.loadApplications()
        }
        .refreshable {
            await viewModel.loadApplications()
        }
    }

    // MARK: - Application List

    private var applicationList: some View {
        List {
            if viewModel.canFilter {
                filterSection
            }

            ForEach(viewModel.filteredApplications) { application in
                ApplicationListRow(application: application)
                    .listRowBackground(Color.tcSurface)
                    .contentShape(Rectangle())
                    .onTapGesture {
                        viewModel.selectApplication(application.id)
                    }
            }
        }
        .listStyle(.plain)
    }

    // MARK: - Filter Section

    private var filterSection: some View {
        Section {
            ScrollView(.horizontal, showsIndicators: false) {
                HStack(spacing: TCSpacing.small) {
                    filterChip(label: "All", status: nil)
                    filterChip(label: "Pending", status: .underReview)
                    filterChip(label: "Approved", status: .approved)
                    filterChip(label: "Refused", status: .refused)
                    filterChip(label: "Withdrawn", status: .withdrawn)
                    filterChip(label: "Appealed", status: .appealed)
                }
                .padding(.horizontal, TCSpacing.medium)
                .padding(.vertical, TCSpacing.small)
            }
        }
        .listRowInsets(EdgeInsets())
        .listRowBackground(Color.tcBackground)
    }

    private func filterChip(label: String, status: ApplicationStatus?) -> some View {
        let isSelected = viewModel.selectedStatusFilter == status
        return Button {
            viewModel.selectedStatusFilter = status
        } label: {
            Text(label)
                .font(TCTypography.captionEmphasis)
                .foregroundStyle(isSelected ? Color.tcTextOnAccent : Color.tcTextPrimary)
                .padding(.horizontal, TCSpacing.small)
                .padding(.vertical, TCSpacing.extraSmall)
                .background(isSelected ? Color.tcAmber : Color.tcSurface)
                .clipShape(Capsule())
                .overlay(
                    Capsule()
                        .stroke(Color.tcBorder, lineWidth: isSelected ? 0 : 1)
                )
        }
        .buttonStyle(.plain)
    }

}
