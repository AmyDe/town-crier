import SwiftUI
import TownCrierDomain

/// The pre-canned filter section (GH#1098): a three-way triad following the
/// exact same structure as `WatchZoneEditorView`'s
/// `shapeModeSection`/`customShapeUpsellPlaceholder`/`lockedCustomShapeSection`.
/// A standalone `View` (like `ZoneMapPreview`/`GatedToggle`) rather than an
/// extension on `WatchZoneEditorView`, so it can hold its own `@ObservedObject`
/// without loosening that type's `viewModel` below `private` -- also keeps
/// `WatchZoneEditorView.swift` under the `file_length` SwiftLint threshold.
struct WatchZoneFilterSection: View {
  @ObservedObject var viewModel: WatchZoneEditorViewModel

  /// A Pro user gets a `NavigationLink` to the picker, a downgraded Pro
  /// holding a filtered zone gets a static locked label, and everyone else
  /// gets the plain-text upsell placeholder.
  var body: some View {
    if viewModel.canSetWatchZoneFilter {
      Section {
        NavigationLink {
          WatchZoneFilterPickerView(viewModel: viewModel)
        } label: {
          HStack {
            Text("Filter")
              .foregroundStyle(Color.tcTextPrimary)
            Spacer()
            Text(viewModel.filterDisplayName(for: viewModel.selectedFilterKey) ?? "None")
              .foregroundStyle(Color.tcTextSecondary)
          }
        }
      } header: {
        Text("Filter")
      } footer: {
        Text("Only see applications that match a pre-set filter, instead of everything nearby.")
          .font(TCTypography.caption)
          .foregroundStyle(Color.tcTextSecondary)
      }
    } else if viewModel.isFilterLocked {
      lockedFilterSection
    } else {
      Section {
        filterUpsellPlaceholder
      }
    }
  }

  /// Shown for a downgraded Pro user editing a zone that already has a
  /// filter set (``WatchZoneEditorViewModel/isFilterLocked``) -- a static
  /// label with no picker affordance, mirroring `lockedCustomShapeSection`'s
  /// visual treatment.
  private var lockedFilterSection: some View {
    Section {
      HStack {
        Text("Filter")
          .foregroundStyle(Color.tcTextPrimary)
        Spacer()
        Text(viewModel.filterDisplayName(for: viewModel.selectedFilterKey) ?? "None")
          .foregroundStyle(Color.tcTextSecondary)
      }
    } header: {
      Text("Filter")
    } footer: {
      Text("This zone's filter is locked. Upgrade to Pro to change it.")
        .font(TCTypography.caption)
        .foregroundStyle(Color.tcTextSecondary)
    }
  }

  /// Minimal upsell affordance for a Free/Personal-tier user, matching
  /// `customShapeUpsellPlaceholder`'s plain-text tap-through treatment.
  private var filterUpsellPlaceholder: some View {
    Button {
      viewModel.viewPlans()
    } label: {
      HStack(spacing: TCSpacing.small) {
        Image(systemName: "line.3.horizontal.decrease.circle.fill")
          .foregroundStyle(Color.tcAmber)
          .accessibilityHidden(true)
        Text("Filter this zone to specific kinds of applications on Pro.")
          .font(TCTypography.caption)
          .foregroundStyle(Color.tcTextSecondary)
          .fixedSize(horizontal: false, vertical: true)
        Spacer(minLength: 0)
        Image(systemName: "chevron.right")
          .font(TCTypography.caption)
          .foregroundStyle(Color.tcTextSecondary)
          .accessibilityHidden(true)
      }
    }
    .buttonStyle(.plain)
    .accessibilityLabel("Filter this zone to specific kinds of applications on Pro.")
    .accessibilityHint("Opens subscription plans")
  }
}
