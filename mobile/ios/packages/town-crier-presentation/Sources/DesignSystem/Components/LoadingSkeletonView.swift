import SwiftUI

/// A shimmer animation modifier for loading skeleton placeholders.
private struct ShimmerModifier: ViewModifier {
  @State private var phase: CGFloat = 0

  func body(content: Content) -> some View {
    content
      .overlay(
        LinearGradient(
          colors: [
            .clear,
            Color.tcTextTertiary.opacity(0.3),
            .clear,
          ],
          startPoint: .leading,
          endPoint: .trailing
        )
        .offset(x: phase)
        .animation(
          .linear(duration: 1.5).repeatForever(autoreverses: false),
          value: phase
        )
      )
      .clipped()
      .onAppear {
        phase = 300
      }
  }
}

/// A rounded rectangle placeholder that shimmers to indicate loading.
public struct SkeletonRow: View {
  private let height: CGFloat

  public init(height: CGFloat = 16) {
    self.height = height
  }

  public var body: some View {
    RoundedRectangle(cornerRadius: TCCornerRadius.small)
      .fill(Color.tcBorder)
      .frame(height: height)
      .modifier(ShimmerModifier())
  }
}

/// Loading skeleton for a card-style layout.
public struct CardSkeletonView: View {
  public init() {}

  public var body: some View {
    VStack(alignment: .leading, spacing: TCSpacing.small) {
      SkeletonRow(height: 12)
        .frame(width: 100)
      SkeletonRow(height: 16)
      SkeletonRow(height: 12)
        .frame(width: 200)
    }
    .padding(TCSpacing.medium)
    .background(Color.tcSurface)
    .clipShape(RoundedRectangle(cornerRadius: TCCornerRadius.medium))
  }
}

/// Loading skeleton for list screens.
public struct ListSkeletonView: View {
  private let rowCount: Int

  public init(rowCount: Int = 5) {
    self.rowCount = rowCount
  }

  public var body: some View {
    VStack(spacing: TCSpacing.small) {
      ForEach(0..<rowCount, id: \.self) { _ in
        CardSkeletonView()
      }
    }
    .padding(.horizontal, TCSpacing.medium)
  }
}

/// Loading skeleton for the map screen.
public struct MapSkeletonView: View {
  public init() {}

  public var body: some View {
    VStack(spacing: TCSpacing.medium) {
      RoundedRectangle(cornerRadius: TCCornerRadius.medium)
        .fill(Color.tcBorder)
        .modifier(ShimmerModifier())

      HStack(spacing: TCSpacing.small) {
        ForEach(0..<3, id: \.self) { _ in
          SkeletonRow(height: 28)
        }
      }
      .padding(.horizontal, TCSpacing.medium)
    }
  }
}
