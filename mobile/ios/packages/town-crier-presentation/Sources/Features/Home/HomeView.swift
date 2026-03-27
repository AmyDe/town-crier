import SwiftUI

/// Placeholder home screen displaying the app title and tagline.
public struct HomeView: View {
    private let viewModel: HomeViewModel

    public init(viewModel: HomeViewModel) {
        self.viewModel = viewModel
    }

    public var body: some View {
        VStack(spacing: TCSpacing.large) {
            Image(systemName: "bell.badge")
                .font(.system(.largeTitle))
                .foregroundStyle(Color.tcAmber)

            Text(viewModel.title)
                .font(.system(.largeTitle, weight: .bold))
                .foregroundStyle(Color.tcTextPrimary)

            Text(viewModel.subtitle)
                .font(.system(.body))
                .foregroundStyle(Color.tcTextSecondary)
        }
        .frame(maxWidth: .infinity, maxHeight: .infinity)
        .background(Color.tcBackground)
    }
}
