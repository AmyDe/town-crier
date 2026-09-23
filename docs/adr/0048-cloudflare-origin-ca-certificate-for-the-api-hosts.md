# 0048. Cloudflare Origin CA certificate for the API hosts

Date: 2026-09-22

## Status

Accepted

## Context

`api.towncrierapp.uk` and `api-dev.towncrierapp.uk` are proxied by Cloudflare (orange cloud, since 2026-06-19, tc-j222), the zone runs SSL mode Full (strict), and the Container Apps ingress admits the 15 Cloudflare address ranges only.

Azure free managed certificates cannot survive that arrangement. Microsoft states two requirements that the setup breaks:

- A subdomain must CNAME directly to the container app. "Mapping to an intermediate CNAME value blocks certificate issuance and renewal. Examples of CNAME values are traffic managers, Cloudflare, and similar services."
- The app must be reachable from the DigiCert validation addresses, which the origin lock forbids.

Renewal therefore failed silently for both hosts. `cert-api-dev` expired on 2026-09-21 and `cert-api-prod` on 2026-09-22, six months after issue. Cloudflare answered every request with 526, so the dev API was unreachable for a day and prod was hours from the same fate. Nothing alerted: the certificates still read `Succeeded`, and the apps themselves stayed healthy.

`share` and `share-dev` are DNS-only, so their managed certificates renewed normally. That contrast isolates the cause.

A TXT-validated managed certificate was tried first and stayed `Pending`, which matches the second requirement above: the validation fetch cannot reach a locked origin.

The options were:

1. Lower the zone to Full (not strict). Cloudflare would accept any origin certificate. Rejected: it removes origin certificate validation, and the certificates would still expire.
2. Unproxy the records so Azure can validate. Rejected: it exposes the origin, defeats the origin lock, and has to be repeated every six months.
3. Serve a Cloudflare Origin CA certificate on the origin.

## Decision

Both API hosts serve a **Cloudflare Origin CA certificate**, valid until 2041-09-18, uploaded to the shared Container Apps environment as `cert-api-origin-ca` and bound `SniEnabled` on `ca-town-crier-api-go-{dev,prod}`.

Cloudflare trusts its own Origin CA on the origin hop, so the zone stays on **Full (strict)**. The certificate covers both hosts through subject alternative names.

`infra/environment.go` no longer creates a managed certificate for the API host. It looks the uploaded certificate up by name and binds it. The private key was generated locally, sent to nobody, and is not in this repository. The certificate signing request alone went to Cloudflare.

The `share` hosts keep their Azure managed certificates. They are DNS-only, so renewal works there.

## Consequences

- Certificate renewal for the API hosts stops depending on Azure and on DNS shape. The next renewal is due in 2041.
- The origin certificate is trusted by Cloudflare only. Direct access to the container app FQDN shows an untrusted certificate. That is already blocked by the origin lock, and it is a further reason not to unproxy these records.
- The certificate is not reproducible from this repository. To replace it, mint a new Origin CA certificate in Cloudflare (a short-lived API token with `SSL and Certificates: Edit` is enough), convert it to `.pfx`, upload it, and rebind. The private key can be regenerated at any time, so losing the current one costs minutes, not access.
- Pulumi deletes `cert-api-dev` and `cert-api-prod` on the next apply of each stack. They are unbound.
- Turning the proxy off for these hosts would now break the origin lock and the certificate story at once. Treat the proxy as a fixed part of the design.
- Expiry is still unmonitored. A certificate-expiry alert is tracked separately, because the failure mode here was silence, not error.
