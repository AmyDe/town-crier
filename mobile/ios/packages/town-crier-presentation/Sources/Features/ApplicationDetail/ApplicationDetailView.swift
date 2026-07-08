import SwiftUI
import TownCrierDomain

#if os(iOS)
  import SafariServices
#endif

/// Full detail view for a planning application.
public struct ApplicationDetailView: View {
  @ObservedObject private var viewModel: ApplicationDetailViewModel
  @State private var showingSafari = false
  @State private var showingShareSheet = false

  public init(viewModel: ApplicationDetailViewModel) {
    _viewModel = ObservedObject(wrappedValue: viewModel)
  }

  public var body: some View {
    ScrollView {
      VStack(alignment: .leading, spacing: TCSpacing.large) {
        StatusBadgeView(status: viewModel.status)

        Text(viewModel.description)
          .font(TCTypography.displaySmall)
          .foregroundStyle(Color.tcTextPrimary)

        detailCard

        if viewModel.hasTimeline {
          VStack(alignment: .leading, spacing: TCSpacing.small) {
            Text("Status Timeline")
              .font(TCTypography.headline)
              .foregroundStyle(Color.tcTextPrimary)

            StatusTimelineView(items: viewModel.timelineItems)
          }
        }

        if viewModel.hasPortalUrl {
          portalButton
        }

        if viewModel.showsSignUpCTA {
          signUpCTA
        }
      }
      .padding(TCSpacing.medium)
    }
    .frame(maxWidth: .infinity, alignment: .leading)
    .background(Color.tcBackground)
    .navigationTitle("Application")
    #if os(iOS)
      .navigationBarTitleDisplayMode(.inline)
      .sheet(isPresented: $showingSafari) {
        if let url = viewModel.portalUrl {
          SafariView(url: url)
        }
      }
      .sheet(isPresented: $showingShareSheet) {
        if let shareURL = viewModel.shareURL {
          ShareSheet(activityItems: [shareURL])
        }
      }
    #endif
    .task {
      await viewModel.loadSavedState()
      // Stale-while-revalidate: the sheet was presented synchronously from
      // the cached row payload, so refresh now to pick up any newer
      // server-side state and to fire `TryRefreshSavedSnapshotAsync` on the
      // backend (bd tc-sslz, tc-udby).
      await viewModel.refresh()
    }
    .toolbar {
      if viewModel.canSave {
        ToolbarItem(placement: .automatic) {
          Button {
            Task { await viewModel.toggleSave() }
          } label: {
            Image(systemName: viewModel.isSaved ? "bookmark.fill" : "bookmark")
              .foregroundStyle(viewModel.isSaved ? Color.tcAmber : Color.tcTextSecondary)
          }
          .accessibilityLabel(viewModel.isSaved ? "Unsave" : "Save")
        }
      }
      // Present the standard iOS share sheet with the canonical public share
      // link (GH #738 Slice 4). Gated on a non-nil `shareURL` so a slug-less
      // (broken) link is never offered — the button appears once `refresh()`
      // has refetched the by-id payload that carries `authoritySlug`.
      if viewModel.shareURL != nil {
        ToolbarItem(placement: .automatic) {
          Button {
            showingShareSheet = true
          } label: {
            Image(systemName: "square.and.arrow.up")
              .foregroundStyle(Color.tcTextSecondary)
          }
          .accessibilityLabel("Share")
        }
      }
    }
  }

  // MARK: - Detail Card

  private var detailCard: some View {
    VStack(alignment: .leading, spacing: TCSpacing.medium) {
      detailRow(icon: "mappin.and.ellipse", label: "Address", value: viewModel.address)
      Divider().foregroundStyle(Color.tcBorder)
      detailRow(icon: "doc.text", label: "Reference", value: viewModel.reference)
      Divider().foregroundStyle(Color.tcBorder)
      detailRow(icon: "building.2", label: "Authority", value: viewModel.authorityName)
      Divider().foregroundStyle(Color.tcBorder)
      detailRow(icon: "calendar", label: "Received", value: viewModel.receivedDateFormatted)
    }
    .padding(TCSpacing.medium)
    .background(Color.tcSurface)
    .clipShape(RoundedRectangle(cornerRadius: TCCornerRadius.medium))
  }

  private func detailRow(icon: String, label: String, value: String) -> some View {
    HStack(alignment: .top, spacing: TCSpacing.small) {
      Image(systemName: icon)
        .font(TCTypography.body)
        .foregroundStyle(Color.tcTextSecondary)
        .frame(width: 24, alignment: .center)

      VStack(alignment: .leading, spacing: TCSpacing.extraSmall) {
        Text(label)
          .font(TCTypography.caption)
          .foregroundStyle(Color.tcTextTertiary)
        Text(value)
          .font(TCTypography.body)
          .foregroundStyle(Color.tcTextPrimary)
      }
    }
  }

  // MARK: - Portal Button

  private var portalButton: some View {
    PrimaryButton {
      // Notify the view model so the coordinator can record the portal-tap
      // review signal (GH #628); the in-app Safari sheet still presents below.
      viewModel.openPortal()
      showingSafari = true
    } label: {
      HStack {
        Image(systemName: "safari")
        Text("View on Council Portal")
      }
    }
  }

  // MARK: - Sign-Up CTA (GH#879 Phase 2)

  /// Replaces the Save toolbar affordance for an anonymously-viewed
  /// application. Reuses ``AccountCTABanner/Copy`` so the wording never drifts
  /// from the anonymous map's CTA — a deliberate product/legal choice
  /// (never say "instant") — while laying out for this screen's already-padded
  /// scroll content rather than the banner's bottom-safe-area pinning.
  private var signUpCTA: some View {
    VStack(alignment: .leading, spacing: TCSpacing.small) {
      Text(AccountCTABanner.Copy.headline)
        .font(TCTypography.headline)
        .foregroundStyle(Color.tcTextPrimary)

      Text(AccountCTABanner.Copy.subline)
        .font(TCTypography.body)
        .foregroundStyle(Color.tcTextSecondary)

      HStack(spacing: TCSpacing.medium) {
        PrimaryButton(AccountCTABanner.Copy.createAccount) {
          viewModel.requestSignUp()
        }

        Button(AccountCTABanner.Copy.signIn) {
          viewModel.requestSignUp()
        }
        .font(TCTypography.bodyEmphasis)
        .foregroundStyle(Color.tcTextSecondary)
      }
    }
    .padding(TCSpacing.medium)
    .background(Color.tcSurfaceElevated)
    .clipShape(RoundedRectangle(cornerRadius: TCCornerRadius.large))
  }

}

#if os(iOS)
  /// Wraps SFSafariViewController for use in SwiftUI.
  struct SafariView: UIViewControllerRepresentable {
    let url: URL

    func makeUIViewController(context: Context) -> SFSafariViewController {
      SFSafariViewController(url: url)
    }

    func updateUIViewController(_ uiViewController: SFSafariViewController, context: Context) {}
  }
#endif
