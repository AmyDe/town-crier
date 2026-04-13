import SwiftUI
import TownCrierDomain

/// The post-onboarding landing screen combining watch zones, authorities, and quick links.
///
/// Matches the web dashboard layout. Each section is a card with a header and content.
/// Available to all tiers -- no entitlement gating required.
public struct DashboardView: View {
  @StateObject private var viewModel: DashboardViewModel

  public init(viewModel: DashboardViewModel) {
    _viewModel = StateObject(wrappedValue: viewModel)
  }

  public var body: some View {
    ScrollView {
      VStack(spacing: TCSpacing.large) {
        header

        if viewModel.isLoading && !viewModel.hasZones {
          loadingState
        } else if let error = viewModel.error, !viewModel.hasZones && !viewModel.hasAuthorities {
          ErrorStateView(error: error) {
            await viewModel.load()
          }
        } else {
          zonesSection
          authoritiesSection
          quickLinksSection
        }
      }
      .padding(.horizontal, TCSpacing.medium)
      .padding(.top, TCSpacing.small)
      .padding(.bottom, TCSpacing.extraLarge)
    }
    .background(Color.tcBackground)
    #if os(iOS)
      .navigationBarTitleDisplayMode(.large)
    #endif
    .navigationTitle("Dashboard")
    .refreshable {
      await viewModel.load()
    }
    .task {
      if !viewModel.hasZones {
        await viewModel.load()
      }
    }
  }

  // MARK: - Header

  private var header: some View {
    HStack {
      VStack(alignment: .leading, spacing: TCSpacing.extraSmall) {
        Text("Town Crier")
          .font(TCTypography.displayLarge)
          .foregroundStyle(Color.tcTextPrimary)

        Text("Planning applications near you")
          .font(TCTypography.body)
          .foregroundStyle(Color.tcTextSecondary)
      }

      Spacer()

      Image(systemName: "bell.badge")
        .font(.system(.title2))
        .foregroundStyle(Color.tcAmber)
    }
    .padding(.bottom, TCSpacing.small)
  }

  // MARK: - Loading

  private var loadingState: some View {
    VStack(spacing: TCSpacing.medium) {
      CardSkeletonView()
      CardSkeletonView()
      CardSkeletonView()
    }
  }

  // MARK: - Watch Zones Section

  private var zonesSection: some View {
    DashboardCard {
      HStack {
        Label("Watch Zones", systemImage: "mappin.and.ellipse")
          .font(TCTypography.headline)
          .foregroundStyle(Color.tcTextPrimary)

        Spacer()

        Button {
          viewModel.navigateToZones()
        } label: {
          Text("View All")
            .font(TCTypography.caption)
            .foregroundStyle(Color.tcAmber)
        }
      }
    } content: {
      if viewModel.hasZones {
        ForEach(viewModel.zones) { zone in
          DashboardZoneRow(zone: zone)
        }
      } else {
        EmptyStateView(
          icon: "mappin.and.ellipse",
          title: "No Watch Zones",
          description: "Add a watch zone to start monitoring planning applications.",
          actionLabel: "Add Watch Zone"
        ) {
          viewModel.navigateToZones()
        }
      }
    }
  }

  // MARK: - Authorities Section

  private var authoritiesSection: some View {
    DashboardCard {
      HStack {
        Label("Authorities", systemImage: "building.2")
          .font(TCTypography.headline)
          .foregroundStyle(Color.tcTextPrimary)

        Spacer()

        Text("\(viewModel.authorityCount)")
          .font(TCTypography.captionEmphasis)
          .foregroundStyle(Color.tcTextSecondary)
      }
    } content: {
      if viewModel.hasAuthorities {
        ForEach(viewModel.authorities, id: \.code) { authority in
          Button {
            viewModel.navigateToAuthority(authority)
          } label: {
            DashboardAuthorityRow(authority: authority)
          }
          .buttonStyle(.plain)
        }
      } else {
        Text("Authorities appear once you add a watch zone.")
          .font(TCTypography.body)
          .foregroundStyle(Color.tcTextSecondary)
          .frame(maxWidth: .infinity, alignment: .leading)
      }
    }
  }

  // MARK: - Quick Links Section

  private var quickLinksSection: some View {
    DashboardCard {
      Label("Quick Links", systemImage: "arrow.right.circle")
        .font(TCTypography.headline)
        .foregroundStyle(Color.tcTextPrimary)
    } content: {
      QuickLinkRow(icon: "bookmark", title: "Saved Applications") {
        viewModel.navigateToSaved()
      }

      QuickLinkRow(icon: "bell", title: "Notifications") {
        viewModel.navigateToNotifications()
      }

      QuickLinkRow(icon: "map", title: "Map") {
        viewModel.navigateToMap()
      }
    }
  }
}

// MARK: - Supporting Views

/// Card container following the design language: surface background, medium radius, standard padding.
private struct DashboardCard<Header: View, Content: View>: View {
  private let header: Header
  private let content: Content

  init(
    @ViewBuilder header: () -> Header,
    @ViewBuilder content: () -> Content
  ) {
    self.header = header()
    self.content = content()
  }

  var body: some View {
    VStack(alignment: .leading, spacing: TCSpacing.medium) {
      header
      content
    }
    .padding(TCSpacing.medium)
    .frame(maxWidth: .infinity, alignment: .leading)
    .background(Color.tcSurface)
    .clipShape(RoundedRectangle(cornerRadius: TCCornerRadius.medium))
  }
}

/// A compact zone summary row for the dashboard.
private struct DashboardZoneRow: View {
  let zone: WatchZone

  var body: some View {
    HStack(spacing: TCSpacing.small) {
      Image(systemName: "mappin.circle.fill")
        .font(.system(.title3))
        .foregroundStyle(Color.tcAmber)

      VStack(alignment: .leading, spacing: TCSpacing.extraSmall) {
        Text(zone.name)
          .font(TCTypography.bodyEmphasis)
          .foregroundStyle(Color.tcTextPrimary)

        Text(formatRadius(zone.radiusMetres))
          .font(TCTypography.caption)
          .foregroundStyle(Color.tcTextSecondary)
      }

      Spacer()
    }
    .padding(.vertical, TCSpacing.extraSmall)
  }
}

/// A row showing an authority name with a chevron.
private struct DashboardAuthorityRow: View {
  let authority: LocalAuthority

  var body: some View {
    HStack(spacing: TCSpacing.small) {
      Image(systemName: "building.2.fill")
        .font(.system(.body))
        .foregroundStyle(Color.tcAmber)

      VStack(alignment: .leading, spacing: TCSpacing.extraSmall) {
        Text(authority.name)
          .font(TCTypography.bodyEmphasis)
          .foregroundStyle(Color.tcTextPrimary)

        if let areaType = authority.areaType {
          Text(areaType)
            .font(TCTypography.caption)
            .foregroundStyle(Color.tcTextSecondary)
        }
      }

      Spacer()

      Image(systemName: "chevron.right")
        .font(TCTypography.caption)
        .foregroundStyle(Color.tcTextTertiary)
    }
    .padding(.vertical, TCSpacing.extraSmall)
  }
}

/// A quick-link row with an icon, title, and chevron.
private struct QuickLinkRow: View {
  let icon: String
  let title: String
  let action: () -> Void

  var body: some View {
    Button(action: action) {
      HStack(spacing: TCSpacing.small) {
        Image(systemName: icon)
          .font(.system(.body))
          .foregroundStyle(Color.tcAmber)
          .frame(width: 24)

        Text(title)
          .font(TCTypography.body)
          .foregroundStyle(Color.tcTextPrimary)

        Spacer()

        Image(systemName: "chevron.right")
          .font(TCTypography.caption)
          .foregroundStyle(Color.tcTextTertiary)
      }
      .padding(.vertical, TCSpacing.small)
    }
    .buttonStyle(.plain)
  }
}
