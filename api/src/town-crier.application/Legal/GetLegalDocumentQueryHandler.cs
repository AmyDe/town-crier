namespace TownCrier.Application.Legal;

public static class GetLegalDocumentQueryHandler
{
    public static Task<GetLegalDocumentResult?> HandleAsync(
        GetLegalDocumentQuery query, CancellationToken ct)
    {
        ArgumentNullException.ThrowIfNull(query);

        var result = query.DocumentType.ToUpperInvariant() switch
        {
            "PRIVACY" => BuildPrivacyPolicy(),
            "TERMS" => BuildTermsOfService(),
            _ => null,
        };

        return Task.FromResult(result);
    }

    private static GetLegalDocumentResult BuildPrivacyPolicy()
    {
        var sections = BuildPrivacyPolicySections();

        return new GetLegalDocumentResult(
            DocumentType: "privacy",
            Title: "Privacy Policy",
            LastUpdated: "2026-03-16",
            Sections: sections);
    }

    private static GetLegalDocumentResult BuildTermsOfService()
    {
        var sections = BuildTermsOfServiceSections();

        return new GetLegalDocumentResult(
            DocumentType: "terms",
            Title: "Terms of Service",
            LastUpdated: "2026-03-16",
            Sections: sections);
    }

    private static IReadOnlyList<LegalDocumentSectionResult> BuildPrivacyPolicySections()
    {
        return
        [
            new LegalDocumentSectionResult(
                Heading: "What We Collect",
                Body: "Town Crier collects the minimum data needed to deliver planning application alerts to you. This includes your postcode or saved locations, notification preferences, and basic device information required for push notifications. If you create an account, we also store your email address for authentication and account recovery."),
            new LegalDocumentSectionResult(
                Heading: "How We Process Your Data",
                Body: "We process your location preferences to match relevant planning applications from local authority data provided by PlanIt (planit.org.uk). Your data is processed on secure servers hosted in Microsoft Azure (UK region). We do not sell, rent, or share your personal data with third parties for marketing purposes."),
            new LegalDocumentSectionResult(
                Heading: "Data Storage and Retention",
                Body: "Your data is stored securely in Azure Cosmos DB within the UK. We retain your data for as long as your account is active. If you stop using Town Crier, your data will be automatically deleted after 12 months of inactivity."),
            new LegalDocumentSectionResult(
                Heading: "Your Rights",
                Body: "Under UK GDPR, you have the right to access, correct, and request deletion of your personal data at any time. You can delete your account and all associated data from within the app settings. You also have the right to data portability and the right to withdraw consent for optional data processing."),
            new LegalDocumentSectionResult(
                Heading: "Push Notifications",
                Body: "Town Crier uses Apple Push Notification Service to deliver planning alerts. You can disable notifications at any time through your device settings or within the app. Disabling notifications does not delete your account or saved preferences."),
            new LegalDocumentSectionResult(
                Heading: "Contact",
                Body: "If you have questions about this privacy policy or wish to exercise your data rights, please contact us at privacy@towncrier.app."),
        ];
    }

    private static IReadOnlyList<LegalDocumentSectionResult> BuildTermsOfServiceSections()
    {
        return
        [
            new LegalDocumentSectionResult(
                Heading: "Acceptance of Terms",
                Body: "By using Town Crier, you agree to these Terms of Service. If you do not agree, please do not use the app. We may update these terms from time to time and will notify you of material changes."),
            new LegalDocumentSectionResult(
                Heading: "Service Description",
                Body: "Town Crier provides notifications about UK local authority planning applications based on your chosen locations. Planning data is sourced from PlanIt (planit.org.uk) and local authority public registers. While we strive for accuracy, we do not guarantee the completeness or timeliness of planning data."),
            new LegalDocumentSectionResult(
                Heading: "Subscriptions",
                Body: "Town Crier offers both free and premium subscription tiers. Premium subscriptions are billed through the Apple App Store. You can manage or cancel your subscription at any time through your App Store account settings. Refunds are handled by Apple in accordance with their refund policy."),
            new LegalDocumentSectionResult(
                Heading: "Acceptable Use",
                Body: "You agree to use Town Crier for its intended purpose of monitoring planning applications. You must not attempt to reverse-engineer the app, scrape data at scale, or use the service to harass or spam other users or planning authorities."),
            new LegalDocumentSectionResult(
                Heading: "Limitation of Liability",
                Body: "Town Crier is provided as-is. We are not liable for decisions made based on planning data shown in the app. Always verify critical planning information directly with your local authority. Our total liability is limited to the amount you have paid for the service in the preceding 12 months."),
            new LegalDocumentSectionResult(
                Heading: "Governing Law",
                Body: "These terms are governed by the laws of England and Wales. Any disputes will be subject to the exclusive jurisdiction of the courts of England and Wales."),
        ];
    }
}
