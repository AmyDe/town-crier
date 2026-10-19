package uk.towncrierapp.domain.devicetoken

/**
 * Removes this device's push-notification token registration from the server,
 * if one was ever registered. Callers treat it as best-effort and swallow failures.
 */
public interface DeviceTokenRepository {
    public suspend fun removeDeviceToken()
}
