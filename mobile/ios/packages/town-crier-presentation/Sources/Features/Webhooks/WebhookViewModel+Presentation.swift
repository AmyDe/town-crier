import Foundation
import TownCrierDomain

/// An action on an existing webhook.
enum WebhookAction: CaseIterable {
  case sendTest
  case rotateSecret
  case remove

  var title: String {
    switch self {
    case .sendTest: WebhookCopy.sendTest
    case .rotateSecret: WebhookCopy.rotateSecret
    case .remove: WebhookCopy.remove
    }
  }
}

enum WebhookStatusTone {
  case active
  case switchedOff
  case paused
}

extension WebhookViewModel {
  var webhook: Webhook? {
    guard case .configured(let webhook) = state else { return nil }
    return webhook
  }

  var showsEditableForm: Bool {
    switch state {
    case .empty: true
    case .configured(let webhook): !webhook.isPaused
    case .loading, .loadFailed, .upsell: false
    }
  }

  var showsViewPlans: Bool {
    state == .upsell || webhook?.isPaused == true
  }

  // The server rejects every other action on a paused webhook.
  var webhookActions: [WebhookAction] {
    guard let webhook else { return [] }
    return webhook.isPaused ? [.remove] : WebhookAction.allCases
  }

  var statusTone: WebhookStatusTone? {
    guard let webhook else { return nil }
    if webhook.isPaused { return .paused }
    return webhook.status == .active ? .active : .switchedOff
  }

  var statusText: String? {
    statusTone.map { tone in
      switch tone {
      case .active: WebhookCopy.statusActive
      case .switchedOff: WebhookCopy.statusSwitchedOff
      case .paused: WebhookCopy.statusPaused
      }
    }
  }

  var switchedOffReason: String? {
    guard statusTone == .switchedOff else { return nil }
    return webhook?.disabledReason == .gone
      ? WebhookCopy.switchedOffGone : WebhookCopy.switchedOffFailures
  }

  var showsDeliveryHistory: Bool {
    webhook?.isPaused == false
  }

  var lastSuccessText: String {
    formatted(webhook?.lastSuccessAt)
  }

  var lastFailureText: String {
    formatted(webhook?.lastFailureAt)
  }

  var lastFailureReasonText: String? {
    guard let reason = webhook?.lastFailureReason else { return nil }
    return WebhookCopy.failureReason(reason, status: webhook?.lastFailureStatus)
  }

  var testResultText: String? {
    testResult.map(WebhookCopy.testResult)
  }

  private func formatted(_ date: Date?) -> String {
    guard let date else { return WebhookCopy.noneYet }
    return date.formattedWithTimeForDisplay(in: timeZone)
  }
}
