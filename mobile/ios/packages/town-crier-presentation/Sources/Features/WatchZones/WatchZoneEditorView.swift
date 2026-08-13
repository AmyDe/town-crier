import SwiftUI
import TownCrierDomain

/// Create or edit a watch zone: postcode entry → radius picker → map preview → save.
public struct WatchZoneEditorView: View {
  @StateObject private var viewModel: WatchZoneEditorViewModel
  @Environment(\.dismiss) private var dismiss

  public init(viewModel: WatchZoneEditorViewModel) {
    _viewModel = StateObject(wrappedValue: viewModel)
  }

  public var body: some View {
    NavigationStack {
      VStack(spacing: 0) {
        errorBanner
        Form {
          nameSection
          if viewModel.isPostcodeFieldVisible {
            postcodeSection
          }
          if viewModel.geocodedCoordinate != nil {
            shapeModeSection
            if viewModel.shapeMode == .circle {
              radiusSection
              mapPreviewSection
            } else if viewModel.canDrawCustomShape {
              boundaryDrawingSection
            } else {
              lockedCustomShapeSection
            }
          }
          if viewModel.areNotificationTogglesVisible {
            notificationsSection
          }
        }
      }
      .navigationTitle(viewModel.isEditing ? "Edit Watch Zone" : "New Watch Zone")
      #if os(iOS)
        .navigationBarTitleDisplayMode(.inline)
      #endif
      .entitlementGateSheet(entitlement: $viewModel.entitlementGate) {
        viewModel.viewPlans()
      }
      .toolbar {
        ToolbarItem(placement: .cancellationAction) {
          Button("Cancel") { dismiss() }
        }
        ToolbarItem(placement: .confirmationAction) {
          Button("Save") {
            Task {
              // Dismiss only on success. On a quota breach the coordinator
              // closes the sheet and opens the paywall (tc-gpjk); on any other
              // failure the editor stays open and shows the inline error.
              if await viewModel.save() {
                dismiss()
              }
            }
          }
          .disabled(
            viewModel.geocodedCoordinate == nil || viewModel.isLoading
              || viewModel.nameInput.trimmingCharacters(in: .whitespaces).isEmpty
              || (viewModel.shapeMode == .custom && !viewModel.canFinishCustomShape)
          )
        }
      }
    }
  }

  private var nameSection: some View {
    Section {
      TextField("e.g. My Home, Office", text: $viewModel.nameInput)
        .autocorrectionDisabled()
    } header: {
      Text("Zone Name")
    } footer: {
      Text("A friendly name to identify this watch zone.")
        .font(TCTypography.caption)
        .foregroundStyle(Color.tcTextSecondary)
    }
  }

  private var postcodeSection: some View {
    Section {
      HStack {
        TextField("Postcode", text: $viewModel.postcodeInput)
          .textContentType(.postalCode)
          .autocorrectionDisabled()
          #if os(iOS)
            .textInputAutocapitalization(.characters)
          #endif

        if viewModel.isLoading {
          ProgressView()
        } else {
          Button("Look up") {
            Task { await viewModel.submitPostcode() }
          }
          .disabled(viewModel.postcodeInput.isEmpty)
        }
      }
    } header: {
      Text("Postcode")
    } footer: {
      Text("Enter a UK postcode to centre your watch zone.")
        .font(TCTypography.caption)
        .foregroundStyle(Color.tcTextSecondary)
    }
  }

  private var radiusSection: some View {
    Section("Radius") {
      VStack(alignment: .leading, spacing: TCSpacing.small) {
        Text(formatRadius(viewModel.selectedRadiusMetres))
          .font(TCTypography.bodyEmphasis)
          .foregroundStyle(Color.tcTextPrimary)

        Slider(
          value: $viewModel.selectedRadiusMetres,
          in: 100...viewModel.maxRadiusMetres,
          step: 100
        )
        .tint(Color.tcAmber)
        .accessibilityLabel("Radius")
        .accessibilityValue(formatRadius(viewModel.selectedRadiusMetres))

        HStack {
          Text(formatRadius(100))
          Spacer()
          Text(formatRadius(viewModel.maxRadiusMetres))
        }
        .font(TCTypography.caption)
        .foregroundStyle(Color.tcTextSecondary)

        if viewModel.canUnlockLargerRadius {
          // Radius upsell (tc-w3cb.3). Routes through viewPlans()/onUpgradeRequired
          // like the instant-alert upsell: the editor closes and the paywall
          // opens. After purchase the list rebuilds on the new tier (.id), so a
          // reopened editor offers the larger range.
          UnlockLargerZonesChip {
            viewModel.viewPlans()
          }
          .padding(.top, TCSpacing.small)
        }

        if viewModel.showsLargeRadiusWarning {
          LargeRadiusWarningView()
            .padding(.top, TCSpacing.small)
        }
      }
    }
  }

  // MARK: - Shape mode (GH#1031)

  /// The Circle/Custom shape control, shown only once a coordinate exists
  /// (mirrors `radiusSection`/`mapPreviewSection`'s visibility gate). Paid
  /// tiers get the segmented control; Free tier sees a minimal upsell
  /// affordance instead (the full onboarding graphic, `CustomShapeUpsellGraphic`,
  /// is a separate bead — tc-6he3x.10 — this is deliberately a plain-text
  /// extension point, not that graphic). Free tier editing an existing
  /// locked custom shape (``WatchZoneEditorViewModel/isCustomShapeLocked``)
  /// gets nothing here — ``lockedCustomShapeSection`` below is the sole,
  /// non-duplicated explanation for that state.
  @ViewBuilder
  private var shapeModeSection: some View {
    if viewModel.canDrawCustomShape {
      Section {
        Picker(
          "Shape",
          selection: Binding(
            get: { viewModel.shapeMode },
            set: { viewModel.selectShapeMode($0) }
          )
        ) {
          Text("Circle").tag(WatchZoneShapeMode.circle)
          Text("Custom").tag(WatchZoneShapeMode.custom)
        }
        .pickerStyle(.segmented)
        .accessibilityLabel("Zone shape")
      }
    } else if !viewModel.isCustomShapeLocked {
      Section {
        customShapeUpsellPlaceholder
      }
    }
  }

  /// Minimal upsell affordance for a Free-tier user who hasn't drawn a
  /// custom shape yet — text plus a "View Plans" tap-through, not the
  /// richer onboarding graphic (tc-6he3x.10).
  private var customShapeUpsellPlaceholder: some View {
    Button {
      viewModel.viewPlans()
    } label: {
      HStack(spacing: TCSpacing.small) {
        Image(systemName: "hexagon.fill")
          .foregroundStyle(Color.tcAmber)
          .accessibilityHidden(true)
        Text("Draw a custom shape on Personal or Pro, instead of just a circle.")
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
    .accessibilityLabel("Draw a custom shape on Personal or Pro, instead of just a circle.")
    .accessibilityHint("Opens subscription plans")
  }

  @ViewBuilder
  private var boundaryDrawingSection: some View {
    if let coordinate = viewModel.geocodedCoordinate {
      Section {
        #if canImport(UIKit)
          BoundaryDrawingMapView(viewModel: viewModel, initialCentre: coordinate)
            .frame(height: 260)
            .clipShape(RoundedRectangle(cornerRadius: TCCornerRadius.medium))
            .listRowInsets(
              EdgeInsets(
                top: TCSpacing.small,
                leading: TCSpacing.medium,
                bottom: TCSpacing.small,
                trailing: TCSpacing.medium
              ))
        #else
          // BoundaryDrawingMapView wraps UIKit's MKMapView and is
          // unavailable on this SPM compile-time-only macOS target (the app
          // ships on iOS; macOS here exists only so `swift build`/`swift
          // test` can exercise the shared code without Xcode).
          Text("Custom-shape drawing is available on iOS.")
            .font(TCTypography.body)
            .foregroundStyle(Color.tcTextSecondary)
        #endif
        HStack {
          Text(vertexCountLabel)
            .font(TCTypography.caption)
            .foregroundStyle(Color.tcTextSecondary)
          Spacer()
          Button("Undo") {
            viewModel.undoLastVertex()
          }
          .disabled(viewModel.boundaryVertices.isEmpty)
        }
      } header: {
        Text("Custom Shape")
      } footer: {
        Text(
          "Tap the map to add points. Drag a point to move it, and tap the first point again to finish."
        )
        .font(TCTypography.caption)
        .foregroundStyle(Color.tcTextSecondary)
      }
    }
  }

  private var vertexCountLabel: String {
    let count = viewModel.boundaryVertices.count
    if count < 3 {
      return "\(count) of at least 3 points"
    }
    return "\(count) points"
  }

  /// Shown instead of ``boundaryDrawingSection`` when editing an existing
  /// custom-shape zone on a tier that can no longer draw one
  /// (``WatchZoneEditorViewModel/isCustomShapeLocked``) — a static preview
  /// with no drawing affordance, so a downgraded Free-tier user has no path
  /// to the vertex editor while the zone's shape stays untouched.
  @ViewBuilder
  private var lockedCustomShapeSection: some View {
    if let coordinate = viewModel.geocodedCoordinate {
      Section {
        ZoneMapPreview(
          centre: coordinate,
          radiusMetres: viewModel.selectedRadiusMetres,
          strokeWidth: 2
        )
        .frame(height: 220)
        .clipShape(RoundedRectangle(cornerRadius: TCCornerRadius.medium))
        .listRowInsets(
          EdgeInsets(
            top: TCSpacing.small,
            leading: TCSpacing.medium,
            bottom: TCSpacing.small,
            trailing: TCSpacing.medium
          ))
      } header: {
        Text("Custom Shape")
      } footer: {
        Text("This zone's custom shape is locked. Upgrade to Personal or Pro to edit it.")
          .font(TCTypography.caption)
          .foregroundStyle(Color.tcTextSecondary)
      }
    }
  }

  private var notificationsSection: some View {
    Section {
      GatedToggle(
        label: "Send push notifications",
        isOn: $viewModel.pushEnabled,
        entitlement: viewModel.instantAlertEntitlement,
        featureGate: viewModel.featureGate
      ) {
        viewModel.requestInstantAlertUpgrade()
      }
      GatedToggle(
        label: "Send instant emails",
        isOn: $viewModel.emailInstantEnabled,
        entitlement: viewModel.instantAlertEntitlement,
        featureGate: viewModel.featureGate
      ) {
        viewModel.requestInstantAlertUpgrade()
      }
    } header: {
      Text("Notifications")
    } footer: {
      Text(notificationsFooterText)
        .font(TCTypography.caption)
        .foregroundStyle(Color.tcTextSecondary)
    }
  }

  private var notificationsFooterText: String {
    if viewModel.featureGate.hasEntitlement(viewModel.instantAlertEntitlement) {
      return "Choose how this zone alerts you when new applications match."
    }
    return "Instant push and email alerts are available on Personal and Pro. "
      + "Free accounts receive a weekly email digest."
  }

  @ViewBuilder
  private var mapPreviewSection: some View {
    if let coordinate = viewModel.geocodedCoordinate {
      Section("Preview") {
        ZoneMapPreview(
          centre: coordinate,
          radiusMetres: viewModel.selectedRadiusMetres,
          strokeWidth: 2
        )
        .frame(height: 220)
        .clipShape(RoundedRectangle(cornerRadius: TCCornerRadius.medium))
        .listRowInsets(
          EdgeInsets(
            top: TCSpacing.small,
            leading: TCSpacing.medium,
            bottom: TCSpacing.small,
            trailing: TCSpacing.medium
          ))
      }
    }
  }

  /// Pinned above the `Form` as a plain `VStack` sibling rather than a
  /// trailing `Form` `Section` (tc-9oyhw, GH#1085) -- a save failure while
  /// scrolled into the custom-shape drawing section with the keyboard open
  /// used to render entirely off-screen. Deliberately NOT a `.safeAreaInset`
  /// on the `Form` (tried first, reverted in tc-p8oks): `Form`/`List` is
  /// UIKit-backed, and its content-inset doesn't reliably track the inset
  /// view's height when that height changes dynamically (e.g. `userMessage`
  /// wrapping to two lines) -- the first `Form` row rendered underneath the
  /// banner's translucent background instead of below it. `MapView`'s
  /// `headerSection` precedent (also a `.safeAreaInset`) doesn't hit this:
  /// it sits over a `ZStack`/`Map`, not a `Form`. A `VStack` sibling lays
  /// out deterministically instead. Always attached so the `if let` inside
  /// collapses to zero size when there's no error -- no new appearance/
  /// disappearance timing versus the removed section conditional.
  @ViewBuilder
  private var errorBanner: some View {
    if let error = viewModel.error {
      HStack(alignment: .top, spacing: TCSpacing.small) {
        Image(systemName: "exclamationmark.triangle.fill")
          .font(TCTypography.body)
          .foregroundStyle(Color.tcStatusRejected)
          .accessibilityHidden(true)

        Text(error.userMessage)
          .font(TCTypography.body)
          .foregroundStyle(Color.tcStatusRejected)
          .fixedSize(horizontal: false, vertical: true)

        Spacer(minLength: 0)
      }
      .padding(TCSpacing.medium)
      .frame(maxWidth: .infinity, alignment: .leading)
      .background(Color.tcStatusRejected.opacity(0.15))
      .clipShape(RoundedRectangle(cornerRadius: TCCornerRadius.medium))
      .padding(.horizontal, TCSpacing.medium)
      .padding(.top, TCSpacing.small)
      .accessibilityElement(children: .combine)
    }
  }
}
