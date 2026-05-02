import SwiftUI
import TownCrierDomain

/// Search screen with authority selector and paginated results.
///
/// Soft-paywall pattern: the search field is visible to all users. Free and
/// personal tier users can type a query, but submitting (or tapping "Search")
/// triggers the entitlement gate sheet via ``SearchViewModel.search`` instead
/// of executing the network call. Pro users execute searches normally.
public struct SearchView: View {
  @StateObject private var viewModel: SearchViewModel
  private let authorities: [LocalAuthority]
  private let onViewPlans: () -> Void

  public init(
    viewModel: SearchViewModel,
    authorities: [LocalAuthority],
    onViewPlans: @escaping () -> Void
  ) {
    _viewModel = StateObject(wrappedValue: viewModel)
    self.authorities = authorities
    self.onViewPlans = onViewPlans
  }

  public var body: some View {
    ZStack {
      Color.tcBackground.ignoresSafeArea()

      VStack(spacing: 0) {
        searchHeader
        resultContent
      }
    }
    .navigationTitle("Search")
    #if os(iOS)
      .navigationBarTitleDisplayMode(.large)
    #endif
    .entitlementGateSheet(entitlement: $viewModel.entitlementGate) {
      onViewPlans()
    }
  }

  private var searchHeader: some View {
    VStack(spacing: TCSpacing.small) {
      // Authority picker
      if !authorities.isEmpty {
        Menu {
          ForEach(authorities, id: \.code) { authority in
            Button(authority.name) {
              viewModel.selectedAuthorityId = Int(authority.code)
            }
          }
        } label: {
          HStack {
            Image(systemName: "building.2")
              .font(TCTypography.body)
            Text(selectedAuthorityName)
              .font(TCTypography.body)
            Spacer()
            Image(systemName: "chevron.down")
              .font(TCTypography.caption)
          }
          .foregroundStyle(Color.tcTextPrimary)
          .padding(TCSpacing.small)
          .background(Color.tcSurface)
          .clipShape(RoundedRectangle(cornerRadius: TCCornerRadius.small))
          .overlay(
            RoundedRectangle(cornerRadius: TCCornerRadius.small)
              .stroke(Color.tcBorder, lineWidth: 1)
          )
        }
      }

      // Search field
      HStack(spacing: TCSpacing.small) {
        Image(systemName: "magnifyingglass")
          .foregroundStyle(Color.tcTextSecondary)

        TextField("Search applications...", text: $viewModel.query)
          .font(TCTypography.body)
          #if os(iOS)
            .textInputAutocapitalization(.never)
            .submitLabel(.search)
          #endif
          .autocorrectionDisabled()
          .onSubmit {
            Task { await viewModel.search() }
          }

        if !viewModel.query.isEmpty {
          Button {
            viewModel.query = ""
          } label: {
            Image(systemName: "xmark.circle.fill")
              .foregroundStyle(Color.tcTextTertiary)
          }
        }
      }
      .padding(TCSpacing.small)
      .background(Color.tcSurface)
      .clipShape(RoundedRectangle(cornerRadius: TCCornerRadius.small))
      .overlay(
        RoundedRectangle(cornerRadius: TCCornerRadius.small)
          .stroke(Color.tcBorder, lineWidth: 1)
      )
    }
    .padding(.horizontal, TCSpacing.medium)
    .padding(.vertical, TCSpacing.small)
  }

  private var selectedAuthorityName: String {
    guard let authorityId = viewModel.selectedAuthorityId else {
      return "Select Authority"
    }
    return authorities.first { $0.code == String(authorityId) }?.name ?? "Unknown Authority"
  }

  @ViewBuilder
  private var resultContent: some View {
    if viewModel.isLoading && viewModel.applications.isEmpty {
      ListSkeletonView()
    } else if let error = viewModel.error {
      ErrorStateView(error: error) {
        await viewModel.search()
      }
    } else if viewModel.isEmpty {
      EmptyStateView(
        icon: "magnifyingglass",
        title: "No Results",
        description: "No planning applications found matching your search."
      )
    } else if !viewModel.hasSearched {
      EmptyStateView(
        icon: "text.magnifyingglass",
        title: "Search Applications",
        description: "Select an authority and enter a search term to find planning applications."
      )
    } else {
      resultList
    }
  }

  private var resultList: some View {
    List {
      // Result count
      Section {
        Text("\(viewModel.total) result\(viewModel.total == 1 ? "" : "s") found")
          .font(TCTypography.caption)
          .foregroundStyle(Color.tcTextSecondary)
      }
      .listRowBackground(Color.tcBackground)

      // Application rows
      ForEach(viewModel.applications) { application in
        ApplicationListRow(application: application)
          .listRowBackground(Color.tcSurface)
          .contentShape(Rectangle())
          .onTapGesture {
            viewModel.selectApplication(application.id)
          }
      }

      // Load more
      if viewModel.hasMore {
        Section {
          HStack {
            Spacer()
            if viewModel.isLoading {
              ProgressView()
            } else {
              Button("Load More") {
                Task { await viewModel.loadMore() }
              }
              .font(TCTypography.bodyEmphasis)
              .foregroundStyle(Color.tcAmber)
            }
            Spacer()
          }
          .frame(minHeight: 44)
        }
        .listRowBackground(Color.tcBackground)
      }
    }
    .listStyle(.plain)
  }

}
