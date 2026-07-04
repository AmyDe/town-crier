package uk.towncrierapp.domain.applications

/**
 * PlanIt's `app_state` vocabulary. Every raw value the wire can send has its
 * own case — including [Unresolved]/[Referred]/[NotAvailable], which decode
 * successfully but are display-grouped with [Unknown] (see `StatusDisplay` in
 * `:presentation`) — so a genuinely unrecognised future value still has
 * somewhere honest to land: [Unknown]. Port of iOS `ApplicationStatus`
 * (GH#775).
 */
public sealed interface ApplicationStatus {
    public data object Undecided : ApplicationStatus

    public data object Permitted : ApplicationStatus

    public data object Conditions : ApplicationStatus

    public data object Rejected : ApplicationStatus

    public data object Withdrawn : ApplicationStatus

    public data object Appealed : ApplicationStatus

    public data object Unresolved : ApplicationStatus

    public data object Referred : ApplicationStatus

    public data object NotAvailable : ApplicationStatus

    /** A raw value outside the known PlanIt vocabulary above. [raw] is preserved for diagnostics. */
    public data class Unknown(
        public val raw: String,
    ) : ApplicationStatus

    public companion object {
        /** Decodes a wire `app_state` value; anything unrecognised (including blank) becomes [Unknown]. */
        public fun fromWireValue(value: String): ApplicationStatus =
            when (value) {
                "Undecided" -> Undecided
                "Permitted" -> Permitted
                "Conditions" -> Conditions
                "Rejected" -> Rejected
                "Withdrawn" -> Withdrawn
                "Appealed" -> Appealed
                "Unresolved" -> Unresolved
                "Referred" -> Referred
                "Not Available" -> NotAvailable
                else -> Unknown(value)
            }
    }
}

/** The wire `app_state` string this status decoded from (or would encode to) — the inverse of [ApplicationStatus.fromWireValue]. */
public val ApplicationStatus.wireValue: String
    get() =
        when (this) {
            ApplicationStatus.Undecided -> "Undecided"
            ApplicationStatus.Permitted -> "Permitted"
            ApplicationStatus.Conditions -> "Conditions"
            ApplicationStatus.Rejected -> "Rejected"
            ApplicationStatus.Withdrawn -> "Withdrawn"
            ApplicationStatus.Appealed -> "Appealed"
            ApplicationStatus.Unresolved -> "Unresolved"
            ApplicationStatus.Referred -> "Referred"
            ApplicationStatus.NotAvailable -> "Not Available"
            is ApplicationStatus.Unknown -> raw
        }

/**
 * Whether a decision has actually been made — the exact set `statusHistory`
 * synthesis (GH#775) uses to decide whether a `decidedDate` becomes a second
 * timeline event, or is folded away because the application isn't actually
 * decided yet (e.g. [ApplicationStatus.Unresolved]).
 */
public val ApplicationStatus.isDecided: Boolean
    get() =
        when (this) {
            ApplicationStatus.Permitted,
            ApplicationStatus.Conditions,
            ApplicationStatus.Rejected,
            ApplicationStatus.Withdrawn,
            ApplicationStatus.Appealed,
            -> true

            ApplicationStatus.Undecided,
            ApplicationStatus.Unresolved,
            ApplicationStatus.Referred,
            ApplicationStatus.NotAvailable,
            is ApplicationStatus.Unknown,
            -> false
        }
