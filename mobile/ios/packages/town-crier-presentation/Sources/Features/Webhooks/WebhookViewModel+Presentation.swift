import Foundation

extension WebhookViewModel {
  var testResultText: String? {
    testResult.map(WebhookCopy.testResult)
  }
}
