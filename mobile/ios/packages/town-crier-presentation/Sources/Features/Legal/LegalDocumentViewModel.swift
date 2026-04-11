/// ViewModel for displaying a legal document (privacy policy or terms of service).
/// All properties are static after initialisation, so this is a plain value type.
public struct LegalDocumentViewModel: Sendable {
  public let title: String
  public let lastUpdated: String
  public let sections: [LegalDocumentSection]
  public let documentType: LegalDocumentType

  public init(documentType: LegalDocumentType) {
    self.documentType = documentType

    switch documentType {
    case .privacyPolicy:
      title = "Privacy Policy"
      lastUpdated = "16 March 2026"
      sections = Self.privacyPolicySections
    case .termsOfService:
      title = "Terms of Service"
      lastUpdated = "16 March 2026"
      sections = Self.termsOfServiceSections
    }
  }
}

// MARK: - Content

extension LegalDocumentViewModel {
  private static var privacyPolicySections: [LegalDocumentSection] {
    [
      LegalDocumentSection(
        heading: "What We Collect",
        body: """
          Town Crier collects the minimum data needed to deliver planning \
          application alerts to you. This includes your postcode or saved \
          locations, notification preferences, and basic device information \
          required for push notifications. If you create an account, we also \
          store your email address for authentication and account recovery.
          """
      ),
      LegalDocumentSection(
        heading: "How We Process Your Data",
        body: """
          We process your location preferences to match relevant planning \
          applications from local authority data provided by PlanIt \
          (planit.org.uk). Your data is processed on secure servers hosted \
          in Microsoft Azure (UK region). We do not sell, rent, or share \
          your personal data with third parties for marketing purposes.
          """
      ),
      LegalDocumentSection(
        heading: "Data Storage and Retention",
        body: """
          Your data is stored securely in Azure Cosmos DB within the UK. \
          We retain your data for as long as your account is active. If you \
          stop using Town Crier, your data will be automatically deleted \
          after 12 months of inactivity.
          """
      ),
      LegalDocumentSection(
        heading: "Your Rights",
        body: """
          Under UK GDPR, you have the right to access, correct, and request \
          deletion of your personal data at any time. You can delete your \
          account and all associated data from within the app settings. You \
          also have the right to data portability and the right to withdraw \
          consent for optional data processing.
          """
      ),
      LegalDocumentSection(
        heading: "Push Notifications",
        body: """
          Town Crier uses Apple Push Notification Service to deliver planning \
          alerts. You can disable notifications at any time through your \
          device settings or within the app. Disabling notifications does not \
          delete your account or saved preferences.
          """
      ),
      LegalDocumentSection(
        heading: "Contact",
        body: """
          If you have questions about this privacy policy or wish to exercise \
          your data rights, please contact us at privacy@towncrier.app.
          """
      ),
    ]
  }

  private static var termsOfServiceSections: [LegalDocumentSection] {
    [
      LegalDocumentSection(
        heading: "Acceptance of Terms",
        body: """
          By using Town Crier, you agree to these Terms of Service. If you \
          do not agree, please do not use the app. We may update these terms \
          from time to time and will notify you of material changes.
          """
      ),
      LegalDocumentSection(
        heading: "Service Description",
        body: """
          Town Crier provides notifications about UK local authority planning \
          applications based on your chosen locations. Planning data is sourced \
          from PlanIt (planit.org.uk) and local authority public registers. \
          While we strive for accuracy, we do not guarantee the completeness \
          or timeliness of planning data.
          """
      ),
      LegalDocumentSection(
        heading: "Subscriptions",
        body: """
          Town Crier offers both free and premium subscription tiers. Premium \
          subscriptions are billed through the Apple App Store. You can manage \
          or cancel your subscription at any time through your App Store \
          account settings. Refunds are handled by Apple in accordance with \
          their refund policy.
          """
      ),
      LegalDocumentSection(
        heading: "Acceptable Use",
        body: """
          You agree to use Town Crier for its intended purpose of monitoring \
          planning applications. You must not attempt to reverse-engineer the \
          app, scrape data at scale, or use the service to harass or spam \
          other users or planning authorities.
          """
      ),
      LegalDocumentSection(
        heading: "Limitation of Liability",
        body: """
          Town Crier is provided as-is. We are not liable for decisions made \
          based on planning data shown in the app. Always verify critical \
          planning information directly with your local authority. Our total \
          liability is limited to the amount you have paid for the service in \
          the preceding 12 months.
          """
      ),
      LegalDocumentSection(
        heading: "Governing Law",
        body: """
          These terms are governed by the laws of England and Wales. Any \
          disputes will be subject to the exclusive jurisdiction of the \
          courts of England and Wales.
          """
      ),
    ]
  }
}
