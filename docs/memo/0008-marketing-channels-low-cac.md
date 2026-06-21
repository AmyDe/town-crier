# 0008. Cost-Effective Marketing Channels for a Sub-£2 ARPU App

Date: 2026-06-21

## Status

Open

## Question

Town Crier is live on the App Store as a solo, bootstrapped, pre-revenue operation. The paid tier is £1.99/mo (max £4.99), so cost of customer acquisition (CAC) must be very low, ideally well under £1-2 per paying user, with a strong preference for zero-marginal-cost organic channels.

Which marketing channels are actually viable given two hard constraints (roughly £50/month of spend, and very little founder time), and which audience should be targeted first?

This memo records a deep-research pass (fan-out web searches with adversarial fact-checking, June 2026). Findings below are split into claims that survived verification and claims that were refuted, so the weak evidence is visible rather than hidden.

## Analysis

### The governing constraint is the unit economics, not the budget

Average revenue per user (ARPU) is ~£1.99 and the product is freemium (free weekly digest, paid instant alerts). Verified 2026 UK Apple Search Ads (ASA) benchmarks are a median cost-per-tap (CPT) of $1.31, a cost-per-acquisition (CPA, i.e. cost per download) of $2.02, and a tap-to-install conversion of 64.7% [Adapty, "Apple Ads benchmarks 2026"]. That ~£1.55 figure is per *install*. Because most installs take the free tier, the cost per *paying* user is several multiples higher: at a 10-15% free-to-paid conversion, paid acquisition costs roughly £10-15 per paying user against £1.99 of revenue.

The conclusion that follows is the spine of this memo: **paid acquisition cannot work as a growth engine at this price point.** The strategy must be overwhelmingly organic. The ~£50/month is best treated not as an ad budget but as money for a single tool or a small keyword-discovery experiment. The real levers are the two things a solo developer actually has: the ability to build passive acquisition into the product and website, and a small amount of well-aimed human effort.

This reframes the founder-time question. At under 1 hour per week the answer is "build it once, let it run". Additional hours (2-3 per week) do not buy more ads; they buy the human-relationship channels that are high-return but cannot be automated.

### Audience ranking (cheapest to acquire, best conversion to £1.99)

1. **Residents/homeowners with a live, specific worry: the best payers.** Someone who has just learned of a development near them has acute, time-bound pain and will readily pay £1.99 for instant alerts. Highest intent and conversion. The difficulty is that they are diffuse and expensive to reach individually, so they are best reached at the moment of concern (via SEO) and in bulk (via community groups).

2. **Community and amenity groups: the cheapest distribution, not the best payer.** Residents' associations, civic societies and conservation groups are few, tightly networked, and one warm introduction reaches hundreds of well-targeted people for free. Treat these as the channel that delivers audience 1, not as subscribers in their own right (a group may want one shared view rather than 200 individual subs).

3. **Property professionals: deprioritise for now.** Highest willingness to pay, but very few of them, and they already buy incumbents (LandInsight, Searchland, Barbour ABI, PlanIt direct). A £1.99 consumer app does not serve their workflow. Revisit only with a dedicated pro tier (see memo 0003); it is a different product, not a marketing channel.

### Channel assessment

**App Store Optimisation (ASO): do this first; ideal for the time constraint.** Verified mechanics: the keyword field is 100 hidden characters; title and subtitle are 30 characters each [Apple developer docs]. Verified rules: never repeat title/subtitle words in the keyword field (Apple indexes a term once, so repetition wastes characters), and strip stop words such as "the/and/with/app" which Apple ignores anyway [multi-source ASO consensus]. Higher ratings genuinely improve keyword ranking; Apple itself confirms ratings influence search ranking [Apple developer docs]. The in-app rating prompt (`SKStoreReviewController`) is OS-capped at three prompts per user per year [Apple], so it should fire after a user receives a useful alert, not on launch. Caveat: several widely-quoted "ASO drives X% of installs" statistics were *refuted* in verification as unsourced blog claims. ASO is worth doing because it is free and compounding, not because of any specific install-lift figure.

**Programmatic SEO: the highest-leverage passive play for a developer.** Templated pages on towncrierapp.uk, one per council/town/postcode ("Planning applications in [town]"), generated from data already ingested. This is a build task (the founder's core skill), after which it runs itself and compounds, and it captures audience 1 at the moment of concern. No verified UK-specific traffic figure was found, so this is a high-confidence strategy with unverified magnitude.

**PR via the Local Democracy Reporting Service (LDRS): the best cheap earned-media route.** Verified: LDRS stories are pooled and republished at no cost by 1,000+ UK outlets including the BBC [Wikipedia, BBC, UK Parliament evidence], via ~165 reporters covering councils nationwide. One reporter writing about "an app that tells residents what is being built near them" can syndicate widely for free. A one-off effort with outsized upside.

**Local press and journalist requests: free routes only.** ResponseSource is the verified UK equivalent of HARO, but its category-based pricing starts at £625 per category per year [ResponseSource], which is unaffordable here. Use the free alternatives: the `#journorequest` hashtag, Qwoted's free tier, and direct pitches to local newsdesks.

**Nextdoor: manual posts only, no broadcast.** Verified: the auto-subscribe "verified organisation" broadcast tier is restricted to councils, police, fire and NHS bodies [Local Government Association], so a commercial founder cannot obtain it. Earlier claims that Nextdoor is too shallow or too geofenced to be useful were *refuted*, so it remains usable as a place to post manually, as a person rather than a broadcaster.

**Parish/town council newsletters: low reach, slow.** Verified with a concrete example: Shinfield Parish posts a printed newsletter to all households but only annually, while the monthly version is opt-in email to a self-selected subset [council website]. The high-reach version is too slow to matter and the frequent version reaches few people. Low priority.

**In-product referral loop: build once, runs forever.** "Forward this alert to a neighbour" / "share what is planned on your street". No verified benchmark, but a near-zero-marginal-cost build that turns the most engaged users (people in an active planning dispute, who naturally rally neighbours) into distribution.

**Apple Search Ads: a keyword-discovery probe, not a growth channel.** The old "ASA Basic needs $5,000/month minimum" claim was *refuted/outdated*; ASA Advanced allows tiny spend. Verified: monthly spend is capped at daily budget times 30.4, so £50/month is about £1.64/day [Apple Ads help]. Utility-app CPAs vary widely, from $0.34-$0.56 at the cheap end to $3.74-$4.31 for document/PDF tools [Adapty 2026]. Installs may be cheap, but per the unit economics above, paying users will not be. Use a small ASA spend to learn which keywords convert, then feed those into free ASO.

### Claims that were refuted (recorded so they are not repeated)

- "App Store search drives 65% of installs" / "ASO drives 27-41% of installs" / various Sensor Tower rank-multiplier statistics: refuted as unsourced blog hype.
- "Double your UK keywords for free via the English (Australia) localisation": refuted; the UK storefront indexes only one locale.
- "Seven five-star reviews to offset one one-star": refuted.
- "ASA Basic requires a $5,000/month minimum": outdated/refuted; ASA Advanced has no such floor.

## Options Considered

Two operating plans, depending on available founder time. The £50/month budget is unchanged between them; the only variable that matters is time.

### Plan A: under 1 hour per week

Pure "build once, compounds" channels, ranked by leverage:

| # | Channel | First concrete step | Time | Expected CAC |
|---|---------|--------------------|------|--------------|
| 1 | ASO pass | Rewrite title/subtitle (30 chars each) and fill the 100-char keyword field with town/planning terms, no repeats, no stop words; add a rating prompt after the first useful alert | ~3h one-off | £0 |
| 2 | Programmatic per-town SEO pages | Build a templated `/planning/[town]` route on towncrierapp.uk from existing data; submit a sitemap | dev build, then ~0 | ~£0 marginal |
| 3 | In-app referral/share | Add "share this application / alert a neighbour" with a deep link | dev build, then ~0 | ~£0 |
| 4 | One LDRS/local-press pitch | Email the local-patch Local Democracy Reporter and newsdesk with the "find out what's being built near you" angle | ~2h one-off | ~£0 |
| 5 | One-time listings | Submit to Product Hunt and a few app directories | ~2h one-off | ~£0 |

Skip at this budget: Facebook-group seeding, Nextdoor posting, Reddit, ongoing content, partnership outreach. All require sustained manual effort that is unavailable at under 1 hour per week.

Spend the £50 on: a month or two of an ASO keyword tool, or a small ASA keyword-discovery probe. Not ongoing ads.

### Plan B: 2-3 hours per week, same £50/month

Keep all of Plan A running passively, and spend the extra time on the relationship channels that are high-return precisely because they cannot be automated:

1. **Hyperlocal seeding at the point of controversy (the biggest unlock).** When a contentious development hits a town, that local Facebook group, Nextdoor feed or subreddit is full of high-intent audience 1. Post as a genuine participant, not a marketer. Critical risk: most local Facebook groups ban self-promotion, so message the admin first and get permission or risk being removed and burning the area.
2. **Partnership outreach to community groups.** ~30 minutes per week emailing civic societies (via the Civic Voice network), CPRE branches and residents' associations: "a free tool that alerts your members to applications in your area". One partnership equals a newsletter blast to hundreds of well-targeted people, and each yes is permanent.
3. **A PR rhythm rather than a single pitch.** Rotate LDRS reporters and local papers across regions; answer free journalist requests.
4. **Newsjacking.** Watch for big local-development stories and reach out or post while they are live. High intent, only catchable by a human.

Net difference: Plan A builds passive infrastructure that brings people in; Plan B adds going to where worried residents already are, at the moment they are worried. If 2-3 hours can be found, hyperlocal seeding around live controversies is where most of them should go, as it is the highest-converting activity on the list.

## Recommendation

Treat marketing as overwhelmingly organic and accept that paid channels cannot hit the CAC target at a £1.99 ARPU. Target residents with a live planning worry, reach them through community groups and at-the-moment SEO, and deprioritise property professionals until a dedicated pro tier exists.

Execute Plan A regardless, because it is near-passive and three of its five items (ASO, programmatic SEO, in-app referral) are developer build tasks that compound. If founder time later allows 2-3 hours per week, layer in Plan B's hyperlocal seeding and community-group partnerships, which are the highest-converting but most time-intensive channels.

Immediate next steps worth turning into beads: (1) the programmatic per-town SEO pages on the existing web frontend, and (2) the in-app referral/share mechanism. Both are one-time builds that unlock the highest-leverage passive channels. The ASO copy rewrite and the LDRS pitch email are low-effort follow-ups that can be drafted separately.

This memo can graduate to an ADR if and when a channel strategy is formally committed to and resourced.
