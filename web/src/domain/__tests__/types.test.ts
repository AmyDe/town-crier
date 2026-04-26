import { describe, it, expect } from "vitest";
import {
  type ApplicationUid,
  type WatchZoneId,
  type AuthorityId,
  asApplicationUid,
  asWatchZoneId,
  asAuthorityId,
  isApplicationStatus,
  isSubscriptionTier,
} from "../types";
import * as types from "../types";

describe("branded type constructors", () => {
  it("creates an ApplicationUid from a string", () => {
    const uid: ApplicationUid = asApplicationUid("APP-001");
    expect(uid).toBe("APP-001");
  });

  it("creates a WatchZoneId from a string", () => {
    const id: WatchZoneId = asWatchZoneId("zone-123");
    expect(id).toBe("zone-123");
  });

  it("creates an AuthorityId from a number", () => {
    const id: AuthorityId = asAuthorityId(42);
    expect(id).toBe(42);
  });
});

describe("type guards", () => {
  describe("isApplicationStatus", () => {
    it("returns true for valid statuses (PlanIt vocabulary)", () => {
      expect(isApplicationStatus("Undecided")).toBe(true);
      expect(isApplicationStatus("Permitted")).toBe(true);
      expect(isApplicationStatus("Conditions")).toBe(true);
      expect(isApplicationStatus("Rejected")).toBe(true);
      expect(isApplicationStatus("Withdrawn")).toBe(true);
      expect(isApplicationStatus("Appealed")).toBe(true);
      expect(isApplicationStatus("Unresolved")).toBe(true);
      expect(isApplicationStatus("Referred")).toBe(true);
      expect(isApplicationStatus("Not Available")).toBe(true);
    });

    it("returns false for legacy/invalid statuses", () => {
      // Legacy vocabulary that PlanIt does not actually use
      expect(isApplicationStatus("Approved")).toBe(false);
      expect(isApplicationStatus("Refused")).toBe(false);
      expect(isApplicationStatus("invalid")).toBe(false);
      expect(isApplicationStatus("")).toBe(false);
      expect(isApplicationStatus(42)).toBe(false);
    });
  });

  describe("isSubscriptionTier", () => {
    it("returns true for valid tiers", () => {
      expect(isSubscriptionTier("Free")).toBe(true);
      expect(isSubscriptionTier("Pro")).toBe(true);
    });

    it("returns false for invalid tiers", () => {
      expect(isSubscriptionTier("Premium")).toBe(false);
      expect(isSubscriptionTier("")).toBe(false);
    });
  });

});

describe("Groups removal", () => {
  it("does not export Group-related symbols", () => {
    const exportedNames = Object.keys(types);
    const groupSymbols = exportedNames.filter(
      (name) => /group|invitation/i.test(name)
    );
    expect(groupSymbols).toEqual([]);
  });
});
