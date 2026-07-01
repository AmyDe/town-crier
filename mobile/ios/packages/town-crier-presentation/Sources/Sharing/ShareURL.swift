import Foundation

/// Builds the canonical public share URL for a planning application:
/// `https://share.towncrierapp.uk/a/{authoritySlug}/{ref}`.
///
/// `ref` is the application's full area-prefixed PlanIt `name`, verbatim — it
/// contains slashes (e.g. `Kingston/25/02755/CLC`), which are preserved as path
/// separators. `authoritySlug` always comes from the API (`authoritySlug` on
/// the detail/by-slug JSON); iOS never computes it (GH #738 Slice 4).
public enum ShareURL {
  /// Origin of the public share surface. Mirrors the web `SHARE_ORIGIN`
  /// constant (`web/scripts/lib/constants.mjs`).
  public static let origin = "https://share.towncrierapp.uk"

  /// Returns the canonical share URL, or `nil` when a component is empty or the
  /// pieces cannot form a valid URL.
  ///
  /// Only unsafe characters in `ref` are percent-encoded; `.urlPathAllowed`
  /// keeps `/`, so the ref's slashes remain path separators. The slug is
  /// already URL-safe.
  public static func build(authoritySlug: String, ref: String) -> URL? {
    guard !authoritySlug.isEmpty, !ref.isEmpty else { return nil }
    let encodedRef =
      ref.addingPercentEncoding(withAllowedCharacters: .urlPathAllowed) ?? ref
    return URL(string: "\(origin)/a/\(authoritySlug)/\(encodedRef)")
  }
}
