/// Where a user sent a shared link. The raw values are the wire format of
/// `POST /v1/share-events`.
public enum ShareDestination: String, Equatable, Sendable, CaseIterable {
  case messages
  case mail
  case whatsapp
  case facebook
  case copy
  case other

  /// Maps a raw share sheet activity type. A `nil` or unlisted type is `.other`.
  public init(activityType: String?) {
    switch activityType {
    case "com.apple.UIKit.activity.Message": self = .messages
    case "com.apple.UIKit.activity.Mail": self = .mail
    case "net.whatsapp.WhatsApp.ShareExtension": self = .whatsapp
    case "com.facebook.Facebook.ShareExtension": self = .facebook
    case "com.apple.UIKit.activity.CopyToPasteboard": self = .copy
    default: self = .other
    }
  }
}
