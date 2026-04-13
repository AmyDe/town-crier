import SwiftUI
import Testing

@testable import TownCrierPresentation

@Suite("Binding+MainThread")
@MainActor
struct BindingMainThreadTests {

  @Test func dispatchingSetOnMain_getReturnsCurrentValue() {
    var value = false
    let base = Binding(get: { value }, set: { value = $0 })

    let sut = base.dispatchingSetOnMain()

    #expect(sut.wrappedValue == false)
  }

  @Test func dispatchingSetOnMain_setUpdatesValue_whenAlreadyOnMainThread() {
    var value = false
    let base = Binding(get: { value }, set: { value = $0 })

    let sut = base.dispatchingSetOnMain()
    sut.wrappedValue = true

    #expect(value == true)
  }

  @Test func dispatchingSetOnMain_setUpdatesValue_fromBackgroundThread() async {
    var value = false
    let base = Binding(get: { value }, set: { value = $0 })
    let sut = base.dispatchingSetOnMain()

    // Simulate StoreKit's behavior: setting the binding from a background thread
    await withCheckedContinuation { (continuation: CheckedContinuation<Void, Never>) in
      DispatchQueue.global(qos: .userInitiated).async {
        sut.wrappedValue = true
        // Give the main queue a chance to process
        DispatchQueue.main.async {
          continuation.resume()
        }
      }
    }

    #expect(value == true)
  }
}
