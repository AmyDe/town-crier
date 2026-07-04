package uk.towncrierapp.domain.devicetoken

/**
 * Removes this device's push-notification token registration from the
 * server, if one was ever registered. Real device-token registration lands
 * with #777; until then no implementation is wired at the composition root,
 * so every caller of this port treats it as a best-effort, swallow-failures
 * step (Settings' sign-out and account-deletion flows) — see the
 * `android-tdd-worker`'s tc-4jjw report for the "no-op today" rationale.
 */
public interface DeviceTokenRepository {
    public suspend fun removeDeviceToken()
}
