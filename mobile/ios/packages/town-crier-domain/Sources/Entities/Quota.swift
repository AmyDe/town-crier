/// A quantitative limit that varies by subscription tier.
///
/// Must remain in sync with the API's `Quota` enum.
public enum Quota: Equatable, Hashable, Sendable {
    case watchZones
}
