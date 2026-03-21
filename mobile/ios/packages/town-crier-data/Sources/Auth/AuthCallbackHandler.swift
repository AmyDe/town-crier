import Auth0
import Foundation

public enum AuthCallbackHandler {
    public static func handle(url: URL) {
        WebAuthentication.resume(with: url)
    }
}
