import SwiftUI
import TownCrierDomain

/// Displays the user's watch zones with add/edit/delete actions.
public struct WatchZoneListView: View {
  @StateObject private var viewModel: WatchZoneListViewModel

  public init(viewModel: WatchZoneListViewModel) {
    _viewModel = StateObject(wrappedValue: viewModel)
  }

  public var body: some View {
    List {
      if viewModel.zones.isEmpty && !viewModel.isLoading {
        emptyState
      } else {
        ForEach(viewModel.zones) { zone in
          WatchZoneRow(zone: zone)
            .contentShape(Rectangle())
            .onTapGesture { viewModel.editZone(zone) }
        }
        .onDelete { indexSet in
          guard let index = indexSet.first else { return }
          let zone = viewModel.zones[index]
          Task { await viewModel.deleteZone(zone) }
        }
      }
    }
    .navigationTitle("Watch Zones")
    .toolbar {
      ToolbarItem(placement: .primaryAction) {
        Button {
          viewModel.addZone()
        } label: {
          Image(systemName: "plus")
        }
      }
    }
    .overlay {
      if viewModel.isLoading {
        ProgressView()
      }
    }
    .task {
      await viewModel.load()
    }
  }

  private var emptyState: some View {
    Section {
      VStack(spacing: TCSpacing.medium) {
        Image(systemName: "mappin.and.ellipse")
          .font(.system(.largeTitle))
          .foregroundStyle(Color.tcTextTertiary)
        Text("No Watch Zones")
          .font(.system(.headline).weight(.semibold))
        Text("Add a watch zone to start monitoring planning applications in your area.")
          .font(.system(.body))
          .foregroundStyle(Color.tcTextSecondary)
          .multilineTextAlignment(.center)
        Button {
          viewModel.addZone()
        } label: {
          Text("Add Watch Zone")
            .font(.system(.body).weight(.semibold))
            .frame(maxWidth: .infinity)
        }
        .buttonStyle(.borderedProminent)
        .tint(Color.tcAmber)
      }
      .padding(.vertical, TCSpacing.extraLarge)
    }
  }
}

private struct WatchZoneRow: View {
  let zone: WatchZone

  var body: some View {
    HStack(spacing: TCSpacing.medium) {
      ZoneMapPreview(centre: zone.centre, radiusMetres: zone.radiusMetres)
        .frame(width: 56, height: 56)
        .clipShape(RoundedRectangle(cornerRadius: TCCornerRadius.small))

      VStack(alignment: .leading, spacing: TCSpacing.extraSmall) {
        Text(zone.postcode.value)
          .font(.system(.headline).weight(.semibold))
        Text(formatRadius(zone.radiusMetres))
          .font(.system(.caption))
          .foregroundStyle(Color.tcTextSecondary)
      }

      Spacer()

      Image(systemName: "chevron.right")
        .font(.system(.caption))
        .foregroundStyle(Color.tcTextTertiary)
    }
    .padding(.vertical, TCSpacing.extraSmall)
  }

}
