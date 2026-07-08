import SwiftUI
import TownCrierDomain

/// The anonymous (pre-signup) Zones tab (GH#879 Phase 4): up to
/// ``DeviceLocalZone/maxZoneCount`` device-local areas with create/edit/
/// delete. Mirrors `WatchZoneListView`'s visual conventions, but every
/// notification affordance and any attempt to add a 4th zone route to a
/// sign-up CTA rather than a real quota/entitlement flow — device-local
/// zones never deliver alerts.
public struct DeviceLocalZoneListView: View {
  @StateObject private var viewModel: DeviceLocalZoneListViewModel

  public init(viewModel: DeviceLocalZoneListViewModel) {
    _viewModel = StateObject(wrappedValue: viewModel)
  }

  public var body: some View {
    List {
      if viewModel.zones.isEmpty {
        emptyState
      } else {
        ForEach(viewModel.zones) { zone in
          DeviceLocalZoneRow(zone: zone) { viewModel.requestAlertsSignUp() }
            .contentShape(Rectangle())
            .onTapGesture { viewModel.editZone(zone) }
        }
        .onDelete { indexSet in
          guard let index = indexSet.first else { return }
          viewModel.deleteZone(viewModel.zones[index])
        }
      }
    }
    .scrollContentBackground(.hidden)
    .background(Color.tcBackground)
    .navigationTitle("Zones")
    .toolbar {
      ToolbarItem(placement: .primaryAction) {
        Button {
          viewModel.addZoneTapped()
        } label: {
          Image(systemName: "plus")
        }
        .accessibilityLabel("Add Area")
      }
    }
    .task {
      viewModel.load()
    }
    .sheet(item: $viewModel.editorTarget) { target in
      DeviceLocalZoneEditorView(viewModel: viewModel.makeEditorViewModel(for: target))
    }
    .sheet(isPresented: $viewModel.isSignUpCTAPresented) {
      DeviceLocalZoneSignUpCTAView(
        onCreateAccount: { viewModel.confirmSignUp() },
        onSignIn: { viewModel.confirmSignUp() },
        onDismiss: { viewModel.dismissSignUpCTA() }
      )
      .selfSizingSheet()
    }
  }

  private var emptyState: some View {
    Section {
      VStack(spacing: TCSpacing.medium) {
        Image(systemName: "mappin.and.ellipse")
          .font(.system(.largeTitle))
          .foregroundStyle(Color.tcTextTertiary)
        Text("No Areas Yet")
          .font(.system(.headline).weight(.semibold))
        Text("Add an area to see nearby planning applications.")
          .font(.system(.body))
          .foregroundStyle(Color.tcTextSecondary)
          .multilineTextAlignment(.center)
        Button {
          viewModel.addZoneTapped()
        } label: {
          Text("Add Area")
            .font(.system(.body).weight(.semibold))
            .frame(maxWidth: .infinity)
        }
        .buttonStyle(.borderedProminent)
        .tint(Color.tcAmber)
      }
      .padding(.vertical, TCSpacing.extraLarge)
    }
    .listRowBackground(Color.tcBackground)
  }
}

private struct DeviceLocalZoneRow: View {
  let zone: DeviceLocalZone
  let onAlertTap: () -> Void

  var body: some View {
    HStack(spacing: TCSpacing.medium) {
      ZoneMapPreview(centre: zone.centre, radiusMetres: zone.radiusMetres)
        .frame(width: 56, height: 56)
        .clipShape(RoundedRectangle(cornerRadius: TCCornerRadius.small))

      VStack(alignment: .leading, spacing: TCSpacing.extraSmall) {
        Text(zone.name)
          .font(.system(.headline).weight(.semibold))
        Text(formatRadius(zone.radiusMetres))
          .font(.system(.caption))
          .foregroundStyle(Color.tcTextSecondary)
      }

      Spacer()

      // Any alert/notification affordance on a zone row is a sign-up CTA —
      // device-local zones never deliver alerts (GH#879 Phase 4).
      Button(action: onAlertTap) {
        Image(systemName: "bell.slash")
          .foregroundStyle(Color.tcTextTertiary)
      }
      .buttonStyle(.plain)
      .accessibilityLabel("Alerts require a free account")

      Image(systemName: "chevron.right")
        .font(.system(.caption))
        .foregroundStyle(Color.tcTextTertiary)
    }
    .padding(.vertical, TCSpacing.extraSmall)
    .listRowBackground(Color.tcSurface)
  }
}
