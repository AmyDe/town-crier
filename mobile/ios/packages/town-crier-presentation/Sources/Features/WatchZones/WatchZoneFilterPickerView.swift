import SwiftUI
import TownCrierDomain

/// Picker for the pre-canned watch-zone filter catalog (GH#1098): "None"
/// plus the 7 `WatchZoneFilterKey` cases, a checkmark on the current
/// selection. Tapping a row selects it and pops back to the editor,
/// following this app's existing push-a-list-picker idiom
/// (`WatchZoneEditorView`'s `Form`/`Section` + `NavigationLink` pattern).
public struct WatchZoneFilterPickerView: View {
  @ObservedObject private var viewModel: WatchZoneEditorViewModel
  @Environment(\.dismiss) private var dismiss

  public init(viewModel: WatchZoneEditorViewModel) {
    self.viewModel = viewModel
  }

  public var body: some View {
    List {
      row(
        displayName: "None",
        description: "Show every application in this zone, with no filter applied.",
        isSelected: viewModel.selectedFilterKey == nil
      ) {
        select(nil)
      }

      ForEach(WatchZoneFilterKey.allCases, id: \.self) { filterKey in
        row(
          displayName: filterKey.displayName,
          description: filterKey.description,
          isSelected: viewModel.selectedFilterKey == filterKey
        ) {
          select(filterKey)
        }
      }
    }
    .navigationTitle("Filter")
    #if os(iOS)
      .navigationBarTitleDisplayMode(.inline)
    #endif
  }

  private func select(_ filterKey: WatchZoneFilterKey?) {
    viewModel.selectFilterKey(filterKey)
    dismiss()
  }

  private func row(
    displayName: String,
    description: String,
    isSelected: Bool,
    action: @escaping () -> Void
  ) -> some View {
    Button(action: action) {
      HStack(alignment: .top, spacing: TCSpacing.small) {
        VStack(alignment: .leading, spacing: TCSpacing.extraSmall) {
          Text(displayName)
            .font(TCTypography.bodyEmphasis)
            .foregroundStyle(Color.tcTextPrimary)
          Text(description)
            .font(TCTypography.caption)
            .foregroundStyle(Color.tcTextSecondary)
            .fixedSize(horizontal: false, vertical: true)
        }
        Spacer(minLength: TCSpacing.small)
        if isSelected {
          Image(systemName: "checkmark")
            .font(TCTypography.body)
            .foregroundStyle(Color.tcAmber)
            .accessibilityHidden(true)
        }
      }
      .contentShape(Rectangle())
    }
    .buttonStyle(.plain)
    .accessibilityLabel("\(displayName). \(description)")
    .accessibilityAddTraits(isSelected ? [.isSelected] : [])
  }
}
