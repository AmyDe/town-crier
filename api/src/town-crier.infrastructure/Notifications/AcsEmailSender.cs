using Azure;
using Azure.Communication.Email;
using Microsoft.Extensions.Logging;
using TownCrier.Application.Notifications;
using TownCrier.Application.Observability;
using TownCrier.Domain.Notifications;

namespace TownCrier.Infrastructure.Notifications;

public sealed class AcsEmailSender : IEmailSender
{
    private const string SenderAddress = "hello@towncrierapp.uk";
    private readonly EmailClient emailClient;
    private readonly ILogger<AcsEmailSender> logger;

    public AcsEmailSender(string connectionString, ILogger<AcsEmailSender> logger)
    {
        this.emailClient = new EmailClient(connectionString);
        this.logger = logger;
    }

    public async Task SendDigestAsync(string userId, string email, IReadOnlyList<WatchZoneDigest> digests, CancellationToken ct)
    {
        var totalCount = digests.Sum(d => d.Notifications.Count);
        var htmlBody = BuildDigestHtml(digests, totalCount);

        var emailMessage = new EmailMessage(
            senderAddress: SenderAddress,
            content: new EmailContent(BuildDigestSubject(totalCount))
            {
                Html = htmlBody,
            },
            recipients: new EmailRecipients([new EmailAddress(email)]));

        try
        {
            await this.emailClient.SendAsync(WaitUntil.Started, emailMessage, ct).ConfigureAwait(false);
            ApiMetrics.EmailsSent.Add(1, new KeyValuePair<string, object?>("email.type", "digest"));
        }
#pragma warning disable CA1031 // Failed emails must not block the handler
        catch (Exception ex)
#pragma warning restore CA1031
        {
            ApiMetrics.EmailsFailed.Add(1, new KeyValuePair<string, object?>("email.type", "digest"));
            EmailLog.DigestSendFailed(this.logger, userId, ex);
        }
    }

    public async Task SendNotificationAsync(string userId, string email, Notification notification, CancellationToken ct)
    {
        ArgumentNullException.ThrowIfNull(notification);
        var htmlBody = BuildNotificationHtml(notification);

        var emailMessage = new EmailMessage(
            senderAddress: SenderAddress,
            content: new EmailContent($"New planning application — {notification.ApplicationAddress}")
            {
                Html = htmlBody,
            },
            recipients: new EmailRecipients([new EmailAddress(email)]));

        try
        {
            await this.emailClient.SendAsync(WaitUntil.Started, emailMessage, ct).ConfigureAwait(false);
            ApiMetrics.EmailsSent.Add(1, new KeyValuePair<string, object?>("email.type", "instant"));
        }
#pragma warning disable CA1031 // Failed emails must not block the handler
        catch (Exception ex)
#pragma warning restore CA1031
        {
            ApiMetrics.EmailsFailed.Add(1, new KeyValuePair<string, object?>("email.type", "instant"));
            EmailLog.NotificationSendFailed(this.logger, userId, ex);
        }
    }

    internal static string BuildDigestSubject(int totalCount)
    {
        return $"Planning update — {totalCount} new applications near you";
    }

    internal static string BuildDigestHtml(IReadOnlyList<WatchZoneDigest> digests, int totalCount)
    {
        var zoneBlocks = string.Join(string.Empty, digests.Select(d =>
        {
            var cards = string.Join(string.Empty, d.Notifications.Select(BuildNotificationCard));

            return $"""
                <tr><td style="padding:16px 0 8px 0;font-size:14px;color:#666;text-transform:uppercase;letter-spacing:0.5px;">
                  📍 {HtmlEncode(d.WatchZoneName)}
                </td></tr>
                {cards}
                """;
        }));

        return $"""
            <!DOCTYPE html>
            <html><head><meta charset="utf-8"><meta name="viewport" content="width=device-width"></head>
            <body style="margin:0;padding:0;background:#f0f0f0;font-family:-apple-system,BlinkMacSystemFont,'Segoe UI',Roboto,sans-serif;">
            <table width="100%" cellpadding="0" cellspacing="0"><tr><td align="center" style="padding:24px;">
            <table width="600" cellpadding="0" cellspacing="0" style="background:#ffffff;border-radius:8px;overflow:hidden;">
              <tr><td style="background:#1a1a2e;padding:24px;text-align:center;">
                <div style="font-size:20px;font-weight:700;color:#ffffff;">Town Crier</div>
                <div style="color:#888;font-size:13px;margin-top:4px;">Live Planning Update</div>
              </td></tr>
              <tr><td style="padding:24px;">
                <table width="100%" cellpadding="0" cellspacing="0">
                  {zoneBlocks}
                </table>
                <table width="100%" cellpadding="0" cellspacing="0" style="margin-top:24px;">
                  <tr><td align="center">
                    <a href="https://towncrierapp.uk" style="display:inline-block;background:#4a6cf7;color:#ffffff;padding:12px 32px;border-radius:6px;text-decoration:none;font-weight:600;">View All in App</a>
                  </td></tr>
                </table>
              </td></tr>
              <tr><td style="padding:16px 24px;text-align:center;color:#999;font-size:12px;border-top:1px solid #eee;">
                {totalCount} new application{(totalCount != 1 ? "s" : string.Empty)} · <a href="https://towncrierapp.uk/settings" style="color:#999;">Unsubscribe</a>
              </td></tr>
            </table>
            </td></tr></table>
            </body></html>
            """;
    }

    private static string BuildNotificationCard(Notification notification)
    {
        var addressLine = HtmlEncode(notification.ApplicationAddress);
        if (notification.EventType == NotificationEventType.DecisionUpdate)
        {
            var label = UkPlanningVocabulary.GetDisplayString(notification.Decision);
            if (!string.IsNullOrEmpty(label))
            {
                addressLine = $"<span style=\"display:inline-block;background:#eef1ff;color:#1a1a2e;font-size:11px;font-weight:700;letter-spacing:0.5px;padding:2px 6px;border-radius:4px;margin-right:6px;\">[{HtmlEncode(label)}]</span>{addressLine}";
            }
        }

        // A row marked with both Zone and Saved sources renders once under the
        // zone section with a small "saved" indicator so the user can see that
        // the application is also on their bookmarks list.
        var savedIndicator = notification.WatchZoneId is not null
            && notification.Sources.HasFlag(NotificationSources.Saved)
                ? "<span data-saved-indicator style=\"display:inline-block;background:#fff3cd;color:#664d03;font-size:11px;font-weight:600;letter-spacing:0.3px;padding:2px 6px;border-radius:4px;margin-left:6px;\">★ saved</span>"
                : string.Empty;

        return $"""
            <tr><td style="padding:0 0 8px 0;">
              <table width="100%" cellpadding="0" cellspacing="0" style="background:#f8f9fa;border-radius:6px;">
                <tr><td style="padding:12px;">
                  <div style="font-weight:600;color:#1a1a2e;">{addressLine}{savedIndicator}</div>
                  <div style="color:#4a6cf7;font-size:13px;">{HtmlEncode(notification.ApplicationType ?? "Planning Application")}</div>
                  <div style="color:#666;font-size:13px;margin-top:4px;">{HtmlEncode(Truncate(notification.ApplicationDescription, 120))}</div>
                </td></tr>
              </table>
            </td></tr>
            """;
    }

    private static string BuildNotificationHtml(Notification notification)
    {
        return $"""
            <!DOCTYPE html>
            <html><head><meta charset="utf-8"><meta name="viewport" content="width=device-width"></head>
            <body style="margin:0;padding:0;background:#f0f0f0;font-family:-apple-system,BlinkMacSystemFont,'Segoe UI',Roboto,sans-serif;">
            <table width="100%" cellpadding="0" cellspacing="0"><tr><td align="center" style="padding:24px;">
            <table width="600" cellpadding="0" cellspacing="0" style="background:#ffffff;border-radius:8px;overflow:hidden;">
              <tr><td style="background:#1a1a2e;padding:24px;text-align:center;">
                <div style="font-size:20px;font-weight:700;color:#ffffff;">Town Crier</div>
                <div style="color:#888;font-size:13px;margin-top:4px;">New Planning Application</div>
              </td></tr>
              <tr><td style="padding:24px;">
                <div style="font-size:18px;font-weight:600;color:#1a1a2e;">{HtmlEncode(notification.ApplicationAddress)}</div>
                <div style="color:#4a6cf7;font-size:14px;margin-top:4px;">{HtmlEncode(notification.ApplicationType ?? "Planning Application")}</div>
                <div style="color:#666;font-size:14px;margin-top:12px;">{HtmlEncode(notification.ApplicationDescription)}</div>
                <table width="100%" cellpadding="0" cellspacing="0" style="margin-top:24px;">
                  <tr><td align="center">
                    <a href="https://towncrierapp.uk" style="display:inline-block;background:#4a6cf7;color:#ffffff;padding:12px 32px;border-radius:6px;text-decoration:none;font-weight:600;">View in App</a>
                  </td></tr>
                </table>
              </td></tr>
              <tr><td style="padding:16px 24px;text-align:center;color:#999;font-size:12px;border-top:1px solid #eee;">
                <a href="https://towncrierapp.uk/settings" style="color:#999;">Manage notifications</a>
              </td></tr>
            </table>
            </td></tr></table>
            </body></html>
            """;
    }

    private static string HtmlEncode(string text)
    {
        return System.Net.WebUtility.HtmlEncode(text);
    }

    private static string Truncate(string text, int maxLength)
    {
        return text.Length <= maxLength ? text : string.Concat(text.AsSpan(0, maxLength - 1), "…");
    }
}
