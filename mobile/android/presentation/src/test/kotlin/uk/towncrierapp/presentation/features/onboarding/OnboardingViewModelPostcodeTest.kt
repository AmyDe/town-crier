package uk.towncrierapp.presentation.features.onboarding

import org.junit.jupiter.api.Test
import org.junit.jupiter.api.extension.ExtendWith
import uk.towncrierapp.domain.auth.DomainError
import uk.towncrierapp.domain.onboarding.FakeOnboardingRepository
import uk.towncrierapp.domain.subscriptions.SubscriptionTier
import uk.towncrierapp.domain.watchzones.FakePostcodeGeocoder
import uk.towncrierapp.domain.watchzones.FakeWatchZoneRepository
import uk.towncrierapp.domain.watchzones.aCoordinate
import uk.towncrierapp.presentation.MainDispatcherExtension
import kotlin.test.assertEquals
import kotlin.test.assertFalse
import kotlin.test.assertIs
import kotlin.test.assertNull
import kotlin.test.assertTrue

/**
 * The postcode step: local format validation (never spends a network call on
 * a garbage string) before `PostcodeGeocoder`, mixed-case/optional-space
 * acceptance, and inline retryable errors from the `DomainError` catalogue
 * (tc-7ttz). Port of iOS `OnboardingViewModelTests`'s postcode-step cases.
 */
@ExtendWith(MainDispatcherExtension::class)
class OnboardingViewModelPostcodeTest {
    private fun makeViewModel(
        geocoder: FakePostcodeGeocoder = FakePostcodeGeocoder(),
        tier: SubscriptionTier = SubscriptionTier.FREE,
    ) = OnboardingViewModel(geocoder, FakeWatchZoneRepository(), FakeOnboardingRepository(), tier)

    @Test
    fun `a garbage postcode is rejected without ever calling the geocoder`() {
        val geocoder = FakePostcodeGeocoder()
        val viewModel = makeViewModel(geocoder)
        viewModel.updatePostcode("NOTAPOSTCODE")

        viewModel.lookUpPostcode()

        assertTrue(geocoder.geocodeCalls.isEmpty())
        assertIs<DomainError.GeocodingFailed>(viewModel.uiState.value.postcodeError)
        assertNull(viewModel.uiState.value.geocodedCoordinate)
    }

    @Test
    fun `a postcode missing its inward code is rejected without calling the geocoder`() {
        val geocoder = FakePostcodeGeocoder()
        val viewModel = makeViewModel(geocoder)
        viewModel.updatePostcode("SW1A1")

        viewModel.lookUpPostcode()

        assertTrue(geocoder.geocodeCalls.isEmpty())
        assertIs<DomainError.GeocodingFailed>(viewModel.uiState.value.postcodeError)
    }

    @Test
    fun `a valid postcode without a space is geocoded`() {
        val geocoder = FakePostcodeGeocoder(Result.success(aCoordinate()))
        val viewModel = makeViewModel(geocoder)
        viewModel.updatePostcode("SW1A1AA")

        viewModel.lookUpPostcode()

        assertEquals(listOf("SW1A1AA"), geocoder.geocodeCalls)
        assertEquals(aCoordinate(), viewModel.uiState.value.geocodedCoordinate)
        assertNull(viewModel.uiState.value.postcodeError)
    }

    @Test
    fun `a valid postcode with a space is geocoded`() {
        val geocoder = FakePostcodeGeocoder(Result.success(aCoordinate()))
        val viewModel = makeViewModel(geocoder)
        viewModel.updatePostcode("SW1A 1AA")

        viewModel.lookUpPostcode()

        assertEquals(listOf("SW1A 1AA"), geocoder.geocodeCalls)
        assertEquals(aCoordinate(), viewModel.uiState.value.geocodedCoordinate)
    }

    @Test
    fun `mixed case input is normalised to upper case before geocoding`() {
        val geocoder = FakePostcodeGeocoder(Result.success(aCoordinate()))
        val viewModel = makeViewModel(geocoder)
        viewModel.updatePostcode("sw1a 1aa")

        viewModel.lookUpPostcode()

        assertEquals(listOf("SW1A 1AA"), geocoder.geocodeCalls)
    }

    @Test
    fun `a remote geocode failure surfaces the DomainError inline and does not advance`() {
        val geocoder = FakePostcodeGeocoder(Result.failure(DomainError.NotFound))
        val viewModel = makeViewModel(geocoder)
        viewModel.updatePostcode("ZZ99 9ZZ")

        viewModel.lookUpPostcode()

        val state = viewModel.uiState.value
        assertEquals(DomainError.NotFound, state.postcodeError)
        assertNull(state.geocodedCoordinate)
        assertFalse(state.isLookingUpPostcode)
    }

    @Test
    fun `a retry after a failure clears the previous error on success`() {
        val geocoder = FakePostcodeGeocoder(Result.failure(DomainError.NotFound))
        val viewModel = makeViewModel(geocoder)
        viewModel.updatePostcode("ZZ99 9ZZ")
        viewModel.lookUpPostcode()

        geocoder.geocodeResult = Result.success(aCoordinate())
        viewModel.lookUpPostcode()

        assertNull(viewModel.uiState.value.postcodeError)
        assertEquals(aCoordinate(), viewModel.uiState.value.geocodedCoordinate)
    }
}
