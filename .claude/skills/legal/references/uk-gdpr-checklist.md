# UK GDPR Transparency Checklist

Used by the `/legal` skill during gap analysis. Each item is a thing a Privacy Policy should disclose under UK GDPR, or a behaviour the service should support. Mark each as **covered**, **partial**, or **missing** against the current policy and codebase.

UK GDPR retains the structure of EU GDPR, so Article numbers match. The Information Commissioner's Office (ICO) is the supervisory authority.

## Table of contents

1. Article 13/14 — Transparency (information given to data subjects)
2. Article 6 — Lawful basis for processing
3. Article 7 — Conditions for consent
4. Articles 15–22 — Data subject rights
5. Chapter V (Articles 44–49) — International transfers
6. Article 5 — Data protection principles
7. Article 25 — Data protection by design
8. Article 32 — Security of processing
9. PECR — Cookies and electronic marketing
10. Children (Article 8)
11. Apple App Store / Google Play privacy disclosures

---

## 1. Article 13/14 — Transparency

A Privacy Policy must contain **all** of the following. Missing any item = "missing". Vague but present = "partial".

- **Identity and contact details** of the controller. For Town Crier: the trading entity's name and a contact address.
- **Contact point for data queries** — an email address users can write to. If a DPO exists, their contact. Small teams often don't need a DPO; a general contact is fine.
- **Purposes of processing** — every distinct purpose, not a vague "to provide our services". E.g., "to send you planning alerts", "to authenticate you", "to operate the service securely".
- **Lawful basis for each purpose** — see section 2.
- **Legitimate interests, if relied on** — which interest, and the balancing test outcome.
- **Recipients / categories of recipients** — named third-party processors, or clear categories. See Town Crier processor list in SKILL.md.
- **International transfers** — country of destination, safeguard (UK IDTA, adequacy, etc.), how to get a copy of the safeguard.
- **Retention period** — how long each category of data is kept, or the criteria used to determine it. "As long as necessary" alone is not enough.
- **Data subject rights** — access, rectification, erasure, restriction, portability, objection, withdraw consent. Must list all applicable rights and how to exercise them.
- **Right to complain to the ICO** — must be explicit, with a link or clear instruction. (ico.org.uk)
- **Whether providing data is a statutory/contractual requirement** — and what happens if the user refuses.
- **Automated decision-making / profiling** — if it happens, disclose and explain. If it doesn't, no mention needed.
- **Source of data** (Article 14 — applies when data is obtained from someone other than the user). Relevant for Town Crier only if we enrich user data from external sources.

## 2. Article 6 — Lawful basis

Every processing activity needs one of six lawful bases. Town Crier's likely bases:

| Activity | Likely basis |
|----------|--------------|
| Creating and maintaining an account | Contract (6(1)(b)) |
| Sending planning alerts the user configured | Contract (6(1)(b)) |
| Authentication / security | Contract + Legitimate interest (6(1)(f)) |
| Paid subscription | Contract (6(1)(b)) |
| Telemetry / crash analytics | Legitimate interest (6(1)(f)) — **must** document a Legitimate Interests Assessment |
| Marketing emails (if any) | Consent (6(1)(a)) — **and** soft opt-in under PECR if applicable |
| Responding to support queries | Legitimate interest |

**Consent is the wrong basis for things the user needs to use the service.** If you can't deliver the service without processing the data, use contract, not consent.

The policy should list the basis per purpose, not as a blanket statement.

## 3. Article 7 — Conditions for consent

If consent is a basis anywhere (cookies, marketing), it must be:
- Freely given (not bundled with service acceptance)
- Specific (purpose-by-purpose)
- Informed (the user knows what they're agreeing to)
- Unambiguous (active opt-in — no pre-ticked boxes)
- Withdrawable as easily as it was given

If a cookie banner or marketing opt-in exists, audit it against these.

## 4. Articles 15–22 — Data subject rights

The policy must disclose each right the user has. In practice:

- **Right of access (Art. 15)** — a "Subject Access Request". Response within 1 month, extendable by 2 months for complex cases.
- **Right to rectification (Art. 16)** — correct inaccurate data.
- **Right to erasure / "right to be forgotten" (Art. 17)** — delete data (subject to exceptions). Account deletion from the app settings is the usual way to exercise this.
- **Right to restriction (Art. 18)** — pause processing.
- **Right to data portability (Art. 20)** — machine-readable export of the data the user provided.
- **Right to object (Art. 21)** — especially to processing based on legitimate interests or for direct marketing.
- **Right to withdraw consent** — where consent is the basis.
- **Rights related to automated decision-making (Art. 22)** — only if we do this. Town Crier probably doesn't.

**Code-level gap check:** For each of these, does the service actually support the right? In particular:
- Account deletion endpoint + UI (Art. 17) — is it wired up end-to-end?
- Data export (Art. 20) — is there a way to get all a user's data out?
- If not, the policy must honestly say "contact us at [email] to exercise this right", and the team must actually honour such a request manually within 1 month. File a code-change bead to build a self-service flow.

## 5. Chapter V — International transfers

If personal data leaves the UK, it needs a Chapter V safeguard. For Town Crier's known processors:

| Processor | Data location | Safeguard |
|-----------|--------------|-----------|
| Auth0 (Okta) | US | UK IDTA or Addendum to EU SCCs — Auth0 publishes the template on their trust site. Disclose this. |
| Apple Push Notification Service | US | Device token only; no direct PII. Still disclose. |
| Apple App Store | US / Ireland | Apple's own safeguards; we're not the controller of the payment data. Disclose for transparency. |
| Azure (Cosmos, Container Apps, ACS, App Insights) | Should be UK region — verify in Pulumi | No transfer if data stays in UK; if a service region isn't UK, disclose. |

Policy must say: (a) transfer happens, (b) safeguard relied on, (c) how the user can get a copy of the safeguard.

## 6. Article 5 — Data protection principles

Not disclosures per se, but the policy should be consistent with these:

- **Lawfulness, fairness, transparency** — covered by the policy itself.
- **Purpose limitation** — don't use data for purposes other than those disclosed.
- **Data minimisation** — collect only what's needed. (Town Crier's "we store generalised coordinates rather than exact addresses" is a minimisation win worth surfacing.)
- **Accuracy** — keep data accurate; let users correct it.
- **Storage limitation** — retention periods.
- **Integrity and confidentiality** — security measures.
- **Accountability** — the controller must be able to demonstrate compliance.

If the policy is internally inconsistent (e.g., says "we minimise data" but also discloses collecting things that aren't needed), flag it.

## 7. Article 25 — Data protection by design

Not disclosed to users, but relevant when auditing the codebase:
- Are defaults privacy-friendly? (e.g., notifications opt-in, not opt-out)
- Are PII fields avoided in logs by default?
- Are database fields encrypted at rest? (Cosmos does this automatically; worth noting.)

## 8. Article 32 — Security of processing

Policy should say, at a high level, what security measures are in place. Don't detail the attack surface, but do say:
- Data encrypted in transit (TLS)
- Data encrypted at rest (Azure default)
- Access controls (Auth0-backed authentication)

Avoid promising specifics that could become untrue (e.g., "256-bit AES" specifically — Azure's implementation detail).

## 9. PECR — Cookies and electronic marketing

The Privacy and Electronic Communications Regulations 2003 (PECR) sits alongside UK GDPR and is enforced by the ICO.

**Cookies rule:** any cookie or similar storage (localStorage, fingerprinting) that is **not strictly necessary** for the service needs **prior informed consent**. Strictly necessary = authentication session, shopping cart, load balancing. Analytics, advertising, preference personalisation = consent needed.

**Marketing emails/SMS rule:** must have consent, OR soft opt-in (existing customer, similar product, easy unsubscribe, mentioned at collection).

**Code-level gap check:**
- Does the web app set any non-essential cookies/localStorage without consent? If so: code change required (build a banner or remove the cookie).
- Does the app send marketing emails? If so, is the opt-in flow correct?
- Transactional emails (planning alerts the user signed up for) are not marketing — no separate consent needed beyond the initial sign-up.

## 10. Article 8 — Children

UK GDPR sets the age of consent for information society services at 13. If the service is likely to be used by children, additional protections apply (Age Appropriate Design Code / "Children's Code" from the ICO).

Town Crier's audience is adults monitoring local planning. Still worth a line in the policy: "Town Crier is not directed at children under 13; we don't knowingly collect data from children under 13."

## 11. App Store privacy disclosures

Apple requires a **Privacy Nutrition Label** in App Store Connect. Whatever the label says must match the policy. Mismatches are an app review risk.

Check:
- Data Types: are all types collected by the app declared? (Location, Identifiers, User Content, Usage Data, Diagnostics…)
- Data Use: "linked to user", "used to track"
- Data collected by third-party SDKs must be included (Auth0, analytics if any)

This isn't UK GDPR, but the `/legal` skill should flag it because inconsistencies cause real pain at review time.

---

## Quick reference — ICO links

- Right to complain: https://ico.org.uk/make-a-complaint/
- Lawful basis interactive tool: https://ico.org.uk/for-organisations/uk-gdpr-guidance-and-resources/lawful-basis/lawful-basis-interactive-guidance-tool/
- International transfers: https://ico.org.uk/for-organisations/uk-gdpr-guidance-and-resources/international-transfers/
- Children's Code: https://ico.org.uk/for-organisations/uk-gdpr-guidance-and-resources/childrens-information/childrens-code-guidance-and-resources/
