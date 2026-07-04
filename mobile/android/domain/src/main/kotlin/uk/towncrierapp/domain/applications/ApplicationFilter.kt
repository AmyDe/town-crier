package uk.towncrierapp.domain.applications

/**
 * The list screen's active chip selection: everything, one specific status,
 * or unread-only. A sum type rather than a `status: ApplicationStatus?` +
 * `unreadOnly: Boolean` pair — "status AND unread" is unrepresentable BY
 * CONSTRUCTION (the server 400s if both are sent; this makes that combination
 * impossible to build in the first place, so no runtime guard is needed).
 * Port of iOS `ApplicationFilter` (GH#775).
 */
public sealed interface ApplicationFilter {
    public data object All : ApplicationFilter

    public data class Status(
        public val status: ApplicationStatus,
    ) : ApplicationFilter

    public data object Unread : ApplicationFilter
}
