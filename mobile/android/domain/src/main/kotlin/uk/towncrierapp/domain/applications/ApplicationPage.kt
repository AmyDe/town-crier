package uk.towncrierapp.domain.applications

/** One page of a zone's application list. A `null` [nextCursor] means this was the last page. Port of iOS `ApplicationPage` (GH#775). */
public data class ApplicationPage(
    public val applications: List<PlanningApplication>,
    public val nextCursor: String? = null,
)
