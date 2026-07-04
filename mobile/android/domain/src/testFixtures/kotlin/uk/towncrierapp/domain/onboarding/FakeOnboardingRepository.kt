package uk.towncrierapp.domain.onboarding

/** Hand-written fake for [OnboardingRepository] - a plain in-memory box standing in for DataStore. */
public class FakeOnboardingRepository(
    public var complete: Boolean = false,
) : OnboardingRepository {
    public val setOnboardingCompleteCalls: MutableList<Boolean> = mutableListOf()

    override suspend fun isOnboardingComplete(): Boolean = complete

    override suspend fun setOnboardingComplete(complete: Boolean) {
        setOnboardingCompleteCalls += complete
        this.complete = complete
    }
}
