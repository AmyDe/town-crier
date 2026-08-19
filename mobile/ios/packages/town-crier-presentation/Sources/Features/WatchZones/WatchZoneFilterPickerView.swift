import SwiftUI
import TownCrierDomain

/// Picker for the pre-canned watch-zone filter catalog (GH#1098, GH#1104,
/// bead tc-m8j90.2): "None" plus the fetched
/// ``WatchZoneEditorViewModel/filterCatalog`` entries, a checkmark on the
/// current selection. Tapping a row selects it and pops back to the editor,
/// following this app's existing push-a-list-picker idiom
/// (`WatchZoneEditorView`'s `Form`/`Section` + `NavigationLink` pattern).
/// Renders fetched data rather than a hardcoded enum's `allCases` -- the
/// entire point of GH#1104 -- so a loading row and a retry row cover the
/// fetch-pending and fetch-failed states.
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

      if viewModel.isLoadingFilterCatalog {
        loadingRow
      } else if viewModel.filterCatalogLoadFailed {
        retryRow
      } else {
        ForEach(viewModel.filterCatalog, id: \.key) { entry in
          row(
            displayName: entry.displayName,
            description: entry.description,
            isSelected: viewModel.selectedFilterKey == entry.key
          ) {
            select(entry.key)
          }
        }
      }
    }
    .navigationTitle("Filter")
    #if os(iOS)
      .navigationBarTitleDisplayMode(.inline)
    #endif
    .task {
      await viewModel.loadFilterCatalogIfNeeded()
    }
  }

  private var loadingRow: some View {
    HStack {
      Spacer()
      ProgressView()
      Spacer()
    }
  }

  private var retryRow: some View {
    Button {
      Task { await viewModel.loadFilterCatalogIfNeeded() }
    } label: {
      Text("Couldn't load filters. Tap to retry.")
        .font(TCTypography.body)
        .foregroundStyle(Color.tcTextSecondary)
    }
  }

  private func select(_ filterKey: String?) {
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
