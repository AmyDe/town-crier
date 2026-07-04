package uk.towncrierapp.domain.devicetoken

/** Hand-written fake for [DeviceTokenRepository]. */
public class FakeDeviceTokenRepository(
    public var removeDeviceTokenResult: Result<Unit> = Result.success(Unit),
) : DeviceTokenRepository {
    public val removeDeviceTokenCalls: MutableList<Unit> = mutableListOf()

    override suspend fun removeDeviceToken() {
        removeDeviceTokenCalls += Unit
        removeDeviceTokenResult.getOrThrow()
    }
}
