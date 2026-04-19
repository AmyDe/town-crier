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
            LastUpdated: "2026-04-19",
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
                Heading: "Who We Are",
                Body: "Town Crier is operated by Amy Salter, a sole trader based in the United Kingdom. We are the data controller for the personal information you give us. For any data query, write to privacy@towncrier.app."),
            new LegalDocumentSectionResult(
                Heading: "What We Collect",
                Body: "We keep the collection small. We store your email address (from sign-in), a unique user ID, and the notification preferences you choose. For each area you want to watch we store a label, coordinates, and a radius — not your exact address. If you subscribe we store an Apple App Store transaction ID so we know your tier; we never see your payment details. On iOS we store a push token so we can deliver notifications to your device."),
            new LegalDocumentSectionResult(
                Heading: "Why We Collect It",
                Body: "We use your data on two lawful bases under UK GDPR. To deliver the service you asked for (Article 6(1)(b), contract): your account, watch zones, preferences, push token, and subscription information. To keep the service reliable and secure (Article 6(1)(f), legitimate interests): authentication and server-side error telemetry. We do not process any data on the basis of consent, we do not make automated decisions with legal effects on you, and we do not send marketing email."),
            new LegalDocumentSectionResult(
                Heading: "Who Processes Your Data for Us",
                Body: "We use Microsoft Azure to host the service. Your account, watch zones, notifications, and email delivery run on Azure services based in the UK (Cosmos DB, Container Apps, Communication Services, Application Insights). Our website is served from Azure Static Web Apps in the European Union; this serves the site's HTML, CSS, and JavaScript only, and none of your account data is stored there. We use Auth0 (an Okta company) for sign-in; Auth0 stores your email address and credentials in the United States. We use Apple's Push Notification Service to deliver iOS alerts, and Apple's App Store to handle subscription billing. We use postcodes.io (UK-hosted) to convert postcodes to coordinates. On the web, map tiles are fetched directly from OpenStreetMap when you view a map. We use PlanIt (planit.org.uk) as our source of planning data; we only read from them, we do not send them any of your information."),
            new LegalDocumentSectionResult(
                Heading: "International Transfers",
                Body: "Some of our processors are based outside the UK. Auth0 stores identity data in the United States; we rely on the UK-US Data Bridge and Auth0's standard contractual clauses as the safeguard. Apple's Push Notification Service and App Store operate in the United States under Apple's own transfer safeguards. Azure Static Web Apps, which serves our website, is in the European Union, which the UK recognises as providing adequate data protection. You can request a copy of the relevant safeguards by writing to privacy@towncrier.app."),
            new LegalDocumentSectionResult(
                Heading: "Cookies and Similar Storage",
                Body: "On the web we store two things in your browser's local storage: your Auth0 authentication tokens, so you stay signed in, and your theme preference (light or dark). Both are strictly necessary for the service you asked for, so we do not ask for consent to use them. We do not set tracking cookies and we do not use third-party analytics. When you view a map, your browser fetches map tiles directly from OpenStreetMap, which receives your IP address."),
            new LegalDocumentSectionResult(
                Heading: "How Long We Keep It",
                Body: "Your notifications and decision alerts are automatically deleted after 90 days. Stale iOS push tokens are deleted after 180 days of inactivity. Your account, watch zones, saved applications, and preferences are kept for as long as your account is active. If you do not use Town Crier for 12 months we will delete your account automatically. You can also delete your account yourself at any time from the app settings, which removes all of your data from our systems and from Auth0."),
            new LegalDocumentSectionResult(
                Heading: "Your Rights Under UK GDPR",
                Body: "You can ask us for a copy of your data (Article 15), correct it (Article 16), delete it (Article 17), receive it in a machine-readable format (Article 20), restrict how we use it (Article 18), or object to how we use it (Article 21). The app and website provide self-service delete and export. For anything else, email privacy@towncrier.app and we will respond within one month. If you think we are handling your data incorrectly, you can complain to the Information Commissioner's Office at ico.org.uk/make-a-complaint or by calling 0303 123 1113."),
            new LegalDocumentSectionResult(
                Heading: "Security",
                Body: "Your data is encrypted in transit with TLS and at rest using Azure's default encryption. Access to our systems uses Auth0 for authentication. We do not log request bodies or passwords."),
            new LegalDocumentSectionResult(
                Heading: "Children, Changes, and Contact",
                Body: "Town Crier is not directed at children under 13 and we do not knowingly collect data from anyone under 13. We may update this policy as the service changes; the \"last updated\" date at the top of this document shows when we last revised it. For any question about your data or this policy, write to privacy@towncrier.app."),
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
