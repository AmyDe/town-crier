import Testing

@testable import TownCrierDomain

@Suite("CrashReporter protocol")
struct CrashReporterTests {
    @Test func start_canBeCalledOnSpy() {
        let spy = SpyCrashReporter()

        spy.start()

        #expect(spy.startCallCount == 1)
    }

    @Test func start_calledMultipleTimes_incrementsCount() {
        let spy = SpyCrashReporter()

        spy.start()
        spy.start()

        #expect(spy.startCallCount == 2)
    }
}
