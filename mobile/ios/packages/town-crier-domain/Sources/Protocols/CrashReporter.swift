/// Port for crash reporting infrastructure.
/// Implementations subscribe to system crash diagnostics and make them available for debugging.
public protocol CrashReporter: Sendable {
    func start()
}
