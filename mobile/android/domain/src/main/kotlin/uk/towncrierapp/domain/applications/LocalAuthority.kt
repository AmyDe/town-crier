package uk.towncrierapp.domain.applications

/**
 * The local planning authority a [PlanningApplication] belongs to. [slug] (the
 * human-readable authority identifier used in share URLs and by-slug detail
 * lookups) is only ever populated on detail reads (`GET
 * /v1/applications/{authority}/{name}`) — list rows leave it `null`. Port of
 * iOS `LocalAuthority` (GH#775).
 */
public data class LocalAuthority(
    public val code: String,
    public val name: String,
    public val areaType: String? = null,
    public val slug: String? = null,
)
