import SwiftUI
import TownCrierDomain

/// Horizontal scrollable pill bar for switching between watch zones.
public struct ZonePickerView: View {
  let zones: [WatchZone]
  let selectedZoneId: WatchZoneId?
  let onSelect: (WatchZone) -> Void

  public init(
    zones: [WatchZone],
    selectedZoneId: WatchZoneId?,
    onSelect: @escaping (WatchZone) -> Void
  ) {
    self.zones = zones
    self.selectedZoneId = selectedZoneId
    self.onSelect = onSelect
  }

  public var body: some View {
    ScrollView(.horizontal, showsIndicators: false) {
      HStack(spacing: TCSpacing.small) {
        ForEach(zones) { zone in
          zoneChip(zone: zone, isSelected: zone.id == selectedZoneId)
        }
      }
      .padding(.horizontal, TCSpacing.medium)
      .padding(.vertical, TCSpacing.small)
    }
  }

  private func zoneChip(zone: WatchZone, isSelected: Bool) -> some View {
    Button {
      onSelect(zone)
    } label: {
      Text(zone.name)
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
