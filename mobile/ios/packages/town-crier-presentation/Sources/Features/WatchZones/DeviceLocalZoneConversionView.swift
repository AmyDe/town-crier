import SwiftUI
import TownCrierDomain

/// Post-signup "Add your other areas" conversion sheet (GH#879 Phase 5).
///
/// Lists the device-local zones (GH#879 Phase 4) the onboarding wizard did
/// not already convert, offering to create them as real watch zones. A
/// single "Add These Areas" action converts every listed zone sequentially,
/// in order; hitting the tier's watch-zone quota mid-list routes to the
/// subscription paywall (handled by the coordinator) and leaves the rest
/// here, untouched. "Not Now" leaves everything in local storage — the
/// authed Zones tab's dismissible row offers this sheet again later.
public struct DeviceLocalZoneConversionView: View {
  @StateObject private var viewModel: DeviceLocalZoneConversionViewModel

  public init(viewModel: DeviceLocalZoneConversionViewModel) {
    _viewModel = StateObject(wrappedValue: viewModel)
  }

  public var body: some View {
    VStack(spacing: 0) {
      header

      List(viewModel.zones) { zone in
        ConversionZoneRow(zone: zone)
      }
      .listStyle(.plain)
      .scrollContentBackground(.hidden)
      .background(Color.tcBackground)
      .frame(minHeight: 120)

      if let error = viewModel.error {
        Text(error.userMessage)
          .font(TCTypography.caption)
          .foregroundStyle(Color.tcStatusRejected)
          .padding(.horizontal, TCSpacing.medium)
          .padding(.top, TCSpacing.small)
      }

      footer
    }
    .background(Color.tcSurfaceElevated)
  }

  private var header: some View {
    VStack(alignment: .leading, spacing: TCSpacing.small) {
      Text("Add your other areas")
        .font(TCTypography.displaySmall)
        .foregroundStyle(Color.tcTextPrimary)
      Text("You saved these areas before signing up. Add them as watch zones to get alerts.")
        .font(TCTypography.body)
        .foregroundStyle(Color.tcTextSecondary)
    }
    .padding(TCSpacing.medium)
  }

  private var footer: some View {
    VStack(spacing: TCSpacing.small) {
      PrimaryButton(viewModel.isConverting ? "Adding…" : "Add These Areas") {
        Task { await viewModel.convertAll() }
      }
      .disabled(viewModel.isConverting || viewModel.zones.isEmpty)

      Button {
        viewModel.dismiss()
      } label: {
        Text("Not Now")
          .font(TCTypography.body)
          .foregroundStyle(Color.tcTextSecondary)
      }
      .frame(minHeight: 44)
      .disabled(viewModel.isConverting)
    }
    .padding(.horizontal, TCSpacing.medium)
    .padding(.bottom, TCSpacing.medium)
  }
}

private struct ConversionZoneRow: View {
  let zone: DeviceLocalZone

  var body: some View {
    HStack(spacing: TCSpacing.medium) {
      ZoneMapPreview(centre: zone.centre, radiusMetres: zone.radiusMetres)
        .frame(width: 48, height: 48)
        .clipShape(RoundedRectangle(cornerRadius: TCCornerRadius.small))

      VStack(alignment: .leading, spacing: TCSpacing.extraSmall) {
        Text(zone.name)
          .font(.system(.headline).weight(.semibold))
        Text(formatRadius(zone.radiusMetres))
          .font(.system(.caption))
          .foregroundStyle(Color.tcTextSecondary)
      }

      Spacer()
    }
    .padding(.vertical, TCSpacing.extraSmall)
    .listRowBackground(Color.tcSurface)
  }
}
