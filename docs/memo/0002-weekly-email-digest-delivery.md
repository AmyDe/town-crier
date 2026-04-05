# 0002. Weekly Email Digest Delivery

Date: 2026-04-02

## Status

Superseded by ADR [0020](../adr/0020-email-notifications-via-acs.md)

## Question

Can we add a weekly email digest feature at near-zero cost for 100s-1000s of users?

## Analysis

The existing notification architecture (ADR 0009) already has a `GenerateWeeklyDigestsCommandHandler` that produces weekly summaries as push notifications via APNs. Adding email as a second delivery channel is the simplest path to email digests.

### Cost Analysis

At 1,000 users sending one email per week = ~4,000 emails/month.

| Service | Free Tier | Monthly Cost at 4K emails |
|---------|-----------|--------------------------|
| **Azure Communication Services (Email)** | 1K emails/mo free | ~$0.75 |
| Brevo (ex-Sendinblue) | 300 emails/day free | $0 |
| Resend | 3K emails/mo free | ~$0 up to 750 users |
| AWS SES | None (outside EC2) | ~$0.40 |
| SendGrid | 100 emails/day free | $0 up to ~750 users |

### Custom Domain

Azure Communication Services Email supports custom sender domains (e.g. `digest@towncrierapp.uk`) at no additional cost. Verification is via DNS records in Cloudflare.

## Options Considered

### 1. Azure Communication Services Email (Recommended)

- Native Azure service, no new vendor
- Custom domain free
- Penny-level costs at our scale
- Infrastructure provisioned via Pulumi alongside existing resources

### 2. Third-Party Service (Brevo, Resend, SendGrid)

- More generous free tiers in some cases
- Adds an external vendor dependency
- Separate account/API key management

### 3. Self-Hosted SMTP

- Rejected outright: deliverability nightmares, IP reputation management, not worth it at any scale

## Recommendation

Use **Azure Communication Services Email** with the `towncrierapp.uk` custom domain. Extend the existing `GenerateWeeklyDigestsCommandHandler` to dispatch emails alongside push notifications via a new `IEmailSender` port, mirroring the existing `IPushNotificationSender` pattern. Provision the ACS resource and email domain via Pulumi.

When this graduates to implementation, it should become an ADR.
