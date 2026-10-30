/// Port for counting a finished share. It takes only the destination: no link
/// code, application or user.
public protocol ShareEventReporter: Sendable {
  /// Sends the event. Failure is ignored, so this method does not throw.
  func report(_ destination: ShareDestination) async
}
