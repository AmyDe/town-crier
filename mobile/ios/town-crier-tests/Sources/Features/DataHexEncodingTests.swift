import Foundation
import Testing

@testable import TownCrierPresentation

@Suite("Data hex encoding")
struct DataHexEncodingTests {
  @Test("empty Data produces empty string")
  func emptyData_producesEmptyString() {
    let data = Data()

    #expect(data.hexEncodedString().isEmpty)
  }

  @Test("single byte 0x00 produces \"00\"")
  func singleZeroByte_producesTwoZeroes() {
    let data = Data([0x00])

    #expect(data.hexEncodedString() == "00")
  }

  @Test("single byte 0xff produces \"ff\" (lowercased)")
  func singleHighByte_producesLowercasedHex() {
    let data = Data([0xFF])

    #expect(data.hexEncodedString() == "ff")
  }

  @Test("multiple bytes are concatenated and lowercased")
  func multipleBytes_concatenatedLowercased() {
    let data = Data([0xDE, 0xAD, 0xBE, 0xEF, 0x01, 0x02])

    #expect(data.hexEncodedString() == "deadbeef0102")
  }

  @Test("32-byte token (typical APNs token length)")
  func apnsTokenSizedData_produces64HexChars() {
    let bytes = Array(repeating: UInt8(0xAB), count: 32)
    let data = Data(bytes)

    let hex = data.hexEncodedString()

    #expect(hex.count == 64)
    #expect(hex == String(repeating: "ab", count: 32))
  }
}
