import SwiftUI
import TownCrierDomain

/// Home-tab push-permission nudge banner (issue #624, Prong 2).
///
/// A passive view: all logic lives in ``PushNudgeViewModel``. Rendered above
/// the Applications list, it appears only when the user is on a paid tier and
/// notifications are not authorized. The single button branches on status —
/// "Turn on" requests the system prompt when `.notDetermined`, "Open Settings"
/// deep-links to iOS Settings when `.denied`.
///
/// Status is loaded on appear and re-read on `scenePhase == .active` so the
/// banner disappears once the user enables notifications in iOS Settings and
/// returns.
public struct PushNudgeBanner: View {
  @StateObject private var viewModel: PushNudgeViewModel
  @Environment(\.scenePhase) private var scenePhase

  public init(viewModel: PushNudgeViewModel) {
    _viewModel = StateObject(wrappedValue: viewModel)
  }

  public var body: some View {
    Group {
      if viewModel.isVisible {
        banner
      }
    }
    .task { await viewModel.load() }
    .onChange(of: scenePhase) { _, newPhase in
      guard newPhase == .active else { return }
      Task { await viewModel.refresh() }
    }
  }

  private var banner: some View {
    HStack(alignment: .top, spacing: TCSpacing.medium) {
      Image(systemName: "bell.badge")
        .font(TCTypography.headline)
        .foregroundStyle(Color.tcAmber)
        .accessibilityHidden(true)

      VStack(alignment: .leading, spacing: TCSpacing.small) {
        Text(viewModel.bodyText)
          .font(TCTypography.body)
          .foregroundStyle(Color.tcTextPrimary)
          .fixedSize(horizontal: false, vertical: true)

        Button {
          Task { await viewModel.primaryAction() }
        } label: {
          Text(viewModel.buttonTitle)
            .font(TCTypography.bodyEmphasis)
            .foregroundStyle(Color.tcAmber)
        }
        .accessibilityLabel(viewModel.buttonTitle)
      }

      Spacer(minLength: 0)
    }
    .padding(TCSpacing.medium)
    .background(Color.tcAmberMuted)
    .clipShape(RoundedRectangle(cornerRadius: TCCornerRadius.medium))
    .padding(.horizontal, TCSpacing.medium)
    .padding(.top, TCSpacing.small)
  }
}
