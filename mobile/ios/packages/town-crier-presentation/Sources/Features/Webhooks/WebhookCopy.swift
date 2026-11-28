import TownCrierDomain

/// Every string on the Webhooks screen. The wording matches the web app.
enum WebhookCopy {
  static let title = "Webhooks"

  static let intro =
    "Send your alerts to your own URL, as signed JSON, a Slack message or a Discord message."
  static let docsLink = "Read the webhook docs"
  static let docsURLString = "https://towncrierapp.uk/developers/webhooks"
  static let upsellHeading = "Send your alerts to Slack, Discord or your own system"
  static let upsellBody =
    "Your planning alerts post to a channel your whole team already reads. "
    + "No one else needs the app."
  static let upsellExampleTitle = "Town Crier: 3 planning updates"
  static let upsellExampleLines = [
    "\u{2022} New near Home: 26/01234/FUL, 1 High Street, Leeds. Single-storey rear extension",
    "\u{2022} Decision (Permitted): 26/01001/HOU, 4 Mill Lane, Leeds. Loft conversion",
    "\u{2022} New near Office: 26/09999/FUL, 9 Park Row, Leeds. Change of use to caf\u{00E9}",
  ]
  static let upsellExampleLabel = "Example Slack message"
  static let upsellFootnote = "Part of Pro, with unlimited watch zones up to 10 km."
  static let seeProPlans = "See Pro plans"
  static let paused =
    "We've paused alerts to this webhook. They'll resume when Pro is active again."
  static let viewPlans = "View Plans"
  static let tryAgain = "Try again"
  static let loading = "Loading your webhook"

  static let urlLabel = "Webhook URL"
  static let urlPlaceholder = "https://"
  static let formatLabel = "Format"
  static let formatJSON = "JSON"
  static let formatSlack = "Slack"
  static let formatDiscord = "Discord"
  static let formatOther = "Other"
  static let save = "Save"

  static let secretLabel = "Signing secret"
  static let copySecret = "Copy"
  static let secretOnce =
    "Copy this now. We only show it once. If you lose it, rotate the secret to get a new one."

  static let statusLabel = "Status"
  static let statusActive = "Active"
  static let statusSwitchedOff = "Switched off"
  static let statusPaused = "Paused"
  static let lastSuccessLabel = "Last success"
  static let lastFailureLabel = "Last failure"
  static let noneYet = "None yet"
  static let switchedOffFailures =
    "We couldn't deliver alerts to your webhook for a day, so we've switched it off. "
    + "To switch it back on, check the address and save it again."
  static let switchedOffGone =
    "Your webhook told us it no longer exists, so we've switched it off. "
    + "To switch it back on, check the address and save it again."

  static let sendTest = "Send test event"
  static let rotateSecret = "Rotate secret"
  static let remove = "Remove"
  static let cancel = "Cancel"
  static let removeAlertTitle = "Remove webhook"
  static let removeAlertMessage =
    "We'll stop sending alerts to this address and delete its signing secret. "
    + "Your alerts still arrive in the app as normal."

  static let errorURLInvalid = "Enter a public https address. Only port 443 works."
  static let errorFormatInvalid = "Choose JSON, Slack or Discord."
  static let errorRequiresPro = "Webhooks need a Pro subscription."
  static let errorCooldown = "Wait 10 seconds before you send another test."
  static let errorNotFound = "That webhook no longer exists."
  static let errorLoad = "We couldn't load your webhook. Try again in a moment."
  static let errorSave = "We couldn't save your webhook. Try again in a moment."
  static let errorRemove = "We couldn't remove your webhook. Try again in a moment."
  static let errorTest = "We couldn't send the test event. Try again in a moment."
  static let errorRotate = "We couldn't rotate the secret. Try again in a moment."

  static func formatName(_ format: WebhookFormat?) -> String {
    switch format {
    case .json: formatJSON
    case .slack: formatSlack
    case .discord: formatDiscord
    case nil: formatOther
    }
  }

  static func failureReason(_ reason: WebhookFailureReason, status: Int?) -> String {
    switch reason {
    case .gone:
      "It answered 410, which means the address no longer exists."
    case .httpStatus:
      status.map { "It answered with status \($0)." } ?? "It answered with an error status."
    case .blockedAddress:
      "That address points somewhere we're not allowed to send to."
    case .timeout:
      "It took too long to answer."
    case .tls:
      "We couldn't make a secure connection. Check its certificate."
    case .connection:
      "We couldn't connect to it."
    }
  }

  static func testResult(_ result: WebhookTestResult) -> String {
    if result.delivered {
      return result.status.map { "Test event delivered (status \($0))." }
        ?? "Test event delivered."
    }
    let reason = result.failureReason.map { failure in
      failureReason(failure, status: result.status)
    }
    guard let reason else { return "Test event not delivered." }
    return "Test event not delivered. \(reason)"
  }
}
