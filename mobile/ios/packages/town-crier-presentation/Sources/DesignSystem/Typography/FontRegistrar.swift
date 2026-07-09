import CoreText
import Foundation

/// Registers the design system's bundled Fraunces fonts with Core Text at
/// process startup (GH#857), so `Font.custom("Fraunces", …)` in
/// ``TCTypography`` resolves without an `Info.plist` `UIAppFonts` entry or
/// any `project.yml`/`.xcodeproj` change — the two static instances
/// (Regular, SemiBold) ship as ordinary SPM resources in `Bundle.module` and
/// are registered programmatically via `CTFontManagerRegisterFontsForURL`.
///
/// The app's composition root (`TownCrierApp.init()`) calls
/// ``registerAll()`` exactly once at launch, before any SwiftUI view renders
/// text in a Fraunces role.
public enum FontRegistrar {
  /// Font resource file names (without extension), bundled under
  /// `DesignSystem/Resources/Fonts`. Both are static (non-variable)
  /// instances of the OFL-licensed Fraunces family, sharing the family name
  /// "Fraunces" so `Font.custom("Fraunces", size:).weight(.semibold)`
  /// resolves the SemiBold face via Core Text's weight-trait matching.
  private static let fontFileNames = ["Fraunces-Regular", "Fraunces-SemiBold"]

  /// Registers every bundled font. Returns whether each named font ended up
  /// registered — `true` covers both "newly registered this call" and
  /// "already registered by an earlier call", so callers never need to
  /// guard against calling this more than once (e.g. a SwiftUI preview and
  /// the live app both starting up in the same process).
  @discardableResult
  public static func registerAll() -> [String: Bool] {
    var results: [String: Bool] = [:]
    for name in fontFileNames {
      results[name] = register(fontNamed: name)
    }
    return results
  }

  /// Registers a single bundled `.ttf` by resource name. Returns `true` if
  /// the font is registered, whether newly or already.
  @discardableResult
  static func register(fontNamed name: String) -> Bool {
    guard let url = Bundle.module.url(forResource: name, withExtension: "ttf") else {
      return false
    }

    var unmanagedError: Unmanaged<CFError>?
    let didRegister = CTFontManagerRegisterFontsForURL(url as CFURL, .process, &unmanagedError)
    if didRegister {
      return true
    }

    // CTFontManagerRegisterFontsForURL reports a font that's already
    // registered (e.g. a second call in the same process) as an error —
    // that's a no-op for our purposes, not a failure.
    if let unmanagedError {
      let error = unmanagedError.takeRetainedValue()
      let code = CFErrorGetCode(error)
      if code == CTFontManagerError.alreadyRegistered.rawValue {
        return true
      }
    }
    return false
  }
}
