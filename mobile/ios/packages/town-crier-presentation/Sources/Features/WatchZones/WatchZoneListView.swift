import SwiftUI
import TownCrierDomain

/// Displays the user's watch zones with add/edit/delete actions.
///
/// When the user has reached their tier's zone limit, the add button is replaced
/// with an ``UpgradeBadgeView`` and tapping it triggers the upgrade flow.
///
/// A zone paused by a subscription downgrade (GH#889 P1/P2) shows a
/// ``PausedZoneBadge`` on its row; tapping the badge routes to the same
/// subscription paywall via ``WatchZoneListViewModel/viewPlans()``.
public struct WatchZoneListView: View {
  @StateObject private var viewModel: WatchZoneListViewModel

  public init(viewModel: WatchZoneListViewModel) {
    _viewModel = StateObject(wrappedValue: viewModel)
  }

  public var body: some View {
    List {
      mastheadRow
      if viewModel.zones.isEmpty && !viewModel.isLoading {
        emptyState
      } else {
        ForEach(viewModel.zones) { zone in
          WatchZoneRow(zone: zone) { viewModel.viewPlans() }
            .cardRowInsets()
            .contentShape(Rectangle())
            .onTapGesture { viewModel.editZone(zone) }
        }
        .onDelete { indexSet in
          guard let index = indexSet.first else { return }
          let zone = viewModel.zones[index]
          Task { await viewModel.deleteZone(zone) }
        }

        if viewModel.showsFreeTierUpsell {
          Section {
            WatchZoneInlineUpsellCard { viewModel.viewPlans() }
              .cardRowInsets()
          }
        }

        if viewModel.showsLocalZoneRow {
          Section {
            UnconvertedLocalZoneRow(
              count: viewModel.unconvertedLocalZones.count,
              onTap: { viewModel.convertLocalZones() },
              onDismiss: { viewModel.presentDiscardConfirmation() }
            )
            .cardRowInsets()
          }
          .alert(
            discardConfirmationTitle,
            isPresented: $viewModel.isDiscardConfirmationPresented
          ) {
            Button("Delete", role: .destructive) {
              viewModel.discardLocalZones()
            }
            Button("Keep for later", role: .cancel) {
              viewModel.dismissLocalZoneRow()
            }
          } message: {
            Text(discardConfirmationMessage)
          }
        }
      }
    }
    .listStyle(.plain)
    .scrollContentBackground(.hidden)
    .background(Color.tcBackground)
    .navigationTitle("Watch Zones")
    .mastheadNavigationBar()
    .toolbar {
      ToolbarItem(placement: .primaryAction) {
        if viewModel.showUpgradeBadge {
          Button {
            viewModel.addZone()
          } label: {
            UpgradeBadgeView()
          }
        } else {
          Button {
            viewModel.addZone()
          } label: {
            Image(systemName: "plus")
          }
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
    .sheet(isPresented: $viewModel.isUpgradePromptPresented) {
      WatchZoneUpsellView(
        valueProposition: viewModel.upgradeValueProposition,
        onViewPlans: { viewModel.viewPlans() },
        onDismiss: { viewModel.dismissUpgradePrompt() }
      )
      .selfSizingSheet()
    }
  }

  // MARK: - Discard confirmation (tc-luq4u)
  //
  // An `.alert`, not a `.confirmationDialog` — on iOS 26 the dialog renders
  // as a compact anchored popover next to the "x" button, and popover
  // presentation drops `.cancel`-role buttons entirely (tap-outside is
  // treated as the cancel affordance), leaving "Keep for later"
  // undiscoverable and unreachable by VoiceOver. An alert renders every
  // button in every presentation style and is the HIG-appropriate container
  // for a destructive confirmation.

  private var discardConfirmationTitle: String {
    viewModel.unconvertedLocalZones.count == 1
      ? "Delete this saved area?"
      : "Delete these saved areas?"
  }

  private var discardConfirmationMessage: String {
    viewModel.unconvertedLocalZones.count == 1
      ? "It only exists on this phone and is not being monitored. Delete it, or keep it to add later."
      : "They only exist on this phone and are not being monitored. Delete them, or keep them to add later."
  }

  // MARK: - Masthead

  private var mastheadRow: some View {
    MastheadView(title: "Watch Zones")
      .padding(.horizontal, TCSpacing.medium)
      .padding(.top, TCSpacing.small)
      .padding(.bottom, TCSpacing.extraSmall)
      .listRowSeparator(.hidden)
      .listRowInsets(EdgeInsets())
      .listRowBackground(Color.tcBackground)
  }

  private var emptyState: some View {
    Section {
      VStack(spacing: TCSpacing.medium) {
        Image(systemName: "mappin.and.ellipse")
          .font(TCTypography.displayLarge)
          .foregroundStyle(Color.tcTextTertiary)
        Text("No Watch Zones")
          .font(TCTypography.headline)
        Text(
          "Add a watch zone to start monitoring planning applications in your area."
        )
        .font(TCTypography.body)
        .foregroundStyle(Color.tcTextSecondary)
        .multilineTextAlignment(.center)
        Button {
          viewModel.addZone()
        } label: {
          Text("Add Watch Zone")
            .font(TCTypography.bodyEmphasis)
            .frame(maxWidth: .infinity)
        }
        .buttonStyle(.borderedProminent)
        .tint(Color.tcAmber)
      }
      .padding(.vertical, TCSpacing.extraLarge)
    }
  }
}

extension View {
  /// Shared row insets for the card-style rows appended beneath the zone
  /// list (the free-tier upsell card and the unconverted-local-zones row) —
  /// clears the default list-row padding so each card fills the section
  /// edge-to-edge with its own `TCSpacing.medium` margin.
  func cardRowInsets() -> some View {
    listRowInsets(
      EdgeInsets(
        top: TCSpacing.medium,
        leading: TCSpacing.medium,
        bottom: TCSpacing.medium,
        trailing: TCSpacing.medium
      )
    )
    .listRowBackground(Color.clear)
    .listRowSeparator(.hidden)
  }
}

private struct WatchZoneRow: View {
  let zone: WatchZone
  let onUpgrade: () -> Void

  var body: some View {
    HStack(spacing: TCSpacing.medium) {
      ZoneMapPreview(centre: zone.centre, radiusMetres: zone.radiusMetres)
        .frame(width: 56, height: 56)
        .clipShape(RoundedRectangle(cornerRadius: TCCornerRadius.small))

      VStack(alignment: .leading, spacing: TCSpacing.extraSmall) {
        // Mono header strip: radius reads as the zone's metadata line,
        // ahead of its name (GH#857) — mirrors the planning-reference strip
        // on ApplicationListRow.
        Text(formatRadius(zone.radiusMetres))
          .font(TCTypography.mono)
          .foregroundStyle(Color.tcTextSecondary)
        Text(zone.name)
          .font(TCTypography.headline)
        if zone.paused {
          PausedZoneBadge(onUpgrade: onUpgrade)
        }
      }

      Spacer()

      Image(systemName: "chevron.right")
        .font(TCTypography.caption)
        .foregroundStyle(Color.tcTextTertiary)
    }
    .padding(TCSpacing.medium)
    .noticeCardStyle()
  }

}

/// Dismissible row surfaced while device-local zones (GH#879 Phase 4) remain
/// unconverted after sign-up. Tapping the body reopens the "Add your other
/// areas" conversion sheet; the trailing "x" opens a delete-confirmation
/// alert (tc-luq4u) offering an explicit "Delete" (permanent) or "Keep for
/// later" (session-only dismissal, reappears next launch while zones still
/// remain) — the row previously had no permanent way to decline.
private struct UnconvertedLocalZoneRow: View {
  let count: Int
  let onTap: () -> Void
  let onDismiss: () -> Void

  private var bodyText: String {
    "\(count) \(count == 1 ? "area" : "areas") from before you signed up"
  }

  var body: some View {
    HStack(alignment: .top, spacing: TCSpacing.medium) {
      Image(systemName: "mappin.and.ellipse")
        .font(TCTypography.headline)
        .foregroundStyle(Color.tcAmber)
        .accessibilityHidden(true)

      VStack(alignment: .leading, spacing: TCSpacing.extraSmall) {
        Text(bodyText)
          .font(TCTypography.body)
          .foregroundStyle(Color.tcTextPrimary)
          .fixedSize(horizontal: false, vertical: true)

        Button(action: onTap) {
          Text("Add them")
            .font(TCTypography.bodyEmphasis)
            .foregroundStyle(Color.tcAmber)
        }
      }

      Spacer(minLength: 0)

      Button(action: onDismiss) {
        Image(systemName: "xmark")
          .font(TCTypography.caption)
          .foregroundStyle(Color.tcTextTertiary)
      }
      .buttonStyle(.plain)
      .accessibilityLabel("Dismiss")
    }
    .padding(TCSpacing.medium)
    .background(Color.tcAmberMuted)
    .clipShape(RoundedRectangle(cornerRadius: TCCornerRadius.medium))
  }
}
