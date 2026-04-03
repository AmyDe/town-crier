# 0003. Ultra Subscription Tier for Property Professionals

Date: 2026-04-02

## Status

Open

## Question

What value-add features could justify a premium "Ultra" subscription tier (£15-25/mo) aimed at property professionals (developers, agents, planning consultants)?

## Analysis

The current tier structure is Free, Personal (£1.99/mo), and Pro (£5.99/mo). Pro gates search. There is room for a premium tier targeting professionals whose workflows revolve around monitoring, due diligence, and lead generation from planning data.

Facebook and Nextdoor were investigated as community sentiment sources. Both are walled gardens with no viable API path for reading public discussion content. Facebook's Group API is restricted to admin management tools only; Nextdoor has no read API at all and actively blocks scraping. These are ruled out.

UK government open data APIs are well-suited as enrichment sources — free, OGL-licensed, and covering property, environment, and heritage data.

## Options Considered

### Feature Set — Two Layers

The recommended structure is a convenience base ("Research Desk") for daily stickiness, plus genuinely unique intelligence features as a competitive moat.

**Research Desk (Convenience)**

1. **Site Intelligence Card** — Enriched panel on each planning application showing last sale price (Land Registry Price Paid, free bulk CSV), EPC rating (Open EPC API, free), flood risk (EA Flood Risk API, free), listed building status (Historic England API, free), and conservation area designation (data.gov.uk / LPA datasets, free). Compresses 30 minutes of tabbing between government websites into one glance.

2. **Planning History** — Full history of all past applications at a specific address, queryable from existing PlanIt data (~20M records). Useful for due diligence: "has this site been refused before?"

3. **Authority Analytics** — Per-authority approval/refusal rates, average decision timelines, breakdowns by application type, regional comparisons. Derived entirely from existing PlanIt data.

**Intelligence Layer (Moat)**

4. **Scraped Public Comments + Sentiment** — Scrape comments from council planning portals (iDox, Uniform, etc.) and surface alongside the application with a sentiment summary. Separate investigation track already underway. High defensibility due to scraping infrastructure complexity.

5. **Decision Prediction** — Statistical model predicting likely outcome and timeline based on authority history, application type, and size. Doesn't require sophisticated ML — even authority-level approval rates by app type would be valuable.

6. **Opportunity Alerts** — Pattern-based signals beyond simple "new application in your zone":
   - Refused-then-resubmitted detector (flags new applications at previously refused addresses)
   - Lapsing permission detector (old permissions with no apparent build activity)
   - Withdrawn pre-application alerts (someone tested the waters and backed off)

7. **Application Change Timeline** — Full state change history for each application over time, built from existing PlanIt polling data (`lastDifferent` tracking). Shows how long each stage took.

8. **Competitor/Agent Activity** — Text parsing of applicant/agent fields in PlanIt data to surface patterns: "This developer submitted 12 applications in your watch zone this year."

### Data Sources

| Source | Cost | Notes |
|--------|------|-------|
| HM Land Registry Price Paid | Free | OGL, monthly bulk CSV |
| Open EPC API | Free | Registration required |
| EA Flood Risk API | Free | Rivers/sea + surface water |
| Historic England API | Free | Listed buildings, scheduled monuments |
| Conservation Areas | Free (varies) | data.gov.uk / individual LPAs |
| PlanIt | Free | Already integrated |
| Council planning portals | Free (scraping) | Separate investigation |

### Ruled Out

- **Facebook / Nextdoor integration** — walled gardens, no viable API or scraping path
- **PDF export / report builder** — deferred, rabbit hole
- **Paid commercial data sources** — not viable until revenue justifies the spend

## Recommendation

This is a strong feature set for a £15-25/mo tier. Research Desk features (1-3) are mostly API integrations and could ship first. Intelligence features (4-8) are higher effort but provide genuine differentiation. Recommend building incrementally: Research Desk first for immediate value, then layering intelligence features over subsequent releases.

No decision needed yet — this memo captures the initial exploration for further refinement.
