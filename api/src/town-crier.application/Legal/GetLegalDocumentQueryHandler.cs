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
            LastUpdated: "2026-04-20",
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
                Body: "Town Crier is operated by Amy Salter, a sole trader based in the United Kingdom. We are the data controller for the personal information you give us. For any data query, write to privacy@towncrierapp.uk."),
            new LegalDocumentSectionResult(
                Heading: "What We Collect",
                Body: "We keep the collection small. We store your email address (from sign-in), a unique user ID, and the notification preferences you choose. For each area you want to watch we store a label, coordinates, and a radius — not your exact address. We keep a short-lived history of the alerts we've sent you (new applications and decisions), and a list of the applications you've saved for later. If you subscribe we store an Apple App Store transaction ID so we know your tier; we never see your payment details. If you redeem an offer code, we record which code you redeemed and when. On iOS we store a push token so we can deliver notifications to your device."),
            new LegalDocumentSectionResult(
                Heading: "Why We Collect It",
                Body: "We use your data on two lawful bases under UK GDPR. To deliver the service you asked for (Article 6(1)(b), contract): your account, watch zones, preferences, push token, and subscription information. To keep the service reliable and secure (Article 6(1)(f), legitimate interests): authentication and server-side error telemetry. We do not process any data on the basis of consent, we do not make automated decisions with legal effects on you, and we do not send marketing email."),
            new LegalDocumentSectionResult(
                Heading: "Who Processes Your Data for Us",
                Body: "We use Microsoft Azure to host the service. Your account, watch zones, notifications, and email delivery run on Azure services based in the UK (Cosmos DB, Container Apps, Communication Services, Application Insights). Our website is served from Azure Static Web Apps in the European Union; this serves the site's HTML, CSS, and JavaScript only, and none of your account data is stored there. We use Auth0 (an Okta company) for sign-in; Auth0 stores your email address and credentials in the United Kingdom. We use Apple's Push Notification Service to deliver iOS alerts, and Apple's App Store to handle subscription billing. We use postcodes.io (UK-hosted) to convert postcodes to coordinates, and the UK Government's Planning Data service (planning.data.gov.uk) to look up conservation areas, listed buildings, and other planning designations near an application — we only send coordinates, no personal data. On the web, map tiles are fetched directly from OpenStreetMap when you view a map. We use PlanIt (planit.org.uk) as our source of planning data; we only read from them, we do not send them any of your information."),
            new LegalDocumentSectionResult(
                Heading: "International Transfers",
                Body: "Most of your data stays in the United Kingdom. Apple's Push Notification Service and App Store operate in the United States under Apple's own transfer safeguards. Azure Static Web Apps, which serves our website, is in the European Union, which the UK recognises as providing adequate data protection. Some of our UK-based processors — notably Microsoft Azure and Auth0 — are subsidiaries of United States companies. Although your data is stored and processed in the UK, a US parent company could in principle receive a legal demand from US authorities. Both providers publish Data Processing Agreements committing to notify us of, and contest, any such demand where legally permitted. You can request a copy of those agreements by writing to privacy@towncrierapp.uk."),
            new LegalDocumentSectionResult(
                Heading: "Cookies and Similar Storage",
                Body: "On the web we store two things in your browser's local storage: your Auth0 authentication tokens, so you stay signed in, and your theme preference (light or dark). Both are strictly necessary for the service you asked for, so we do not ask for consent to use them. We do not set tracking cookies and we do not use third-party analytics. When you view a map, your browser fetches map tiles directly from OpenStreetMap, which receives your IP address."),
            new LegalDocumentSectionResult(
                Heading: "How Long We Keep It",
                Body: "Your notifications and decision alerts are automatically deleted after 90 days. We remove iOS push tokens when Apple tells us they're no longer valid. Everything else — your account, watch zones, saved applications, preferences, and subscription status — is kept for as long as your account exists. You can delete your account at any time from the app settings, which removes your profile, preferences, watch zones, notifications, saved applications, and device registrations from our systems."),
            new LegalDocumentSectionResult(
                Heading: "Your Rights Under UK GDPR",
                Body: "You can ask us for a copy of your data (Article 15), correct it (Article 16), delete it (Article 17), receive it in a machine-readable format (Article 20), restrict how we use it (Article 18), or object to how we use it (Article 21). The app provides self-service account deletion. For a full copy of your data, or to exercise any other right, email privacy@towncrierapp.uk and we will respond within one month. If you think we are handling your data incorrectly, you can complain to the Information Commissioner's Office at ico.org.uk/make-a-complaint or by calling 0303 123 1113."),
            new LegalDocumentSectionResult(
                Heading: "Security",
                Body: "Your data is encrypted in transit with TLS and at rest using Azure's default encryption. Access to our systems uses Auth0 for authentication. We do not log request bodies or passwords."),
            new LegalDocumentSectionResult(
                Heading: "Children, Changes, and Contact",
                Body: "Town Crier is not directed at children under 13 and we do not knowingly collect data from anyone under 13. We may update this policy as the service changes; the \"last updated\" date at the top of this document shows when we last revised it. For any question about your data or this policy, write to privacy@towncrierapp.uk."),
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
