import { describe, it, expect } from "vitest";
import {
  type ApplicationUid,
  type WatchZoneId,
  type GroupId,
  type InvitationId,
  type AuthorityId,
  asApplicationUid,
  asWatchZoneId,
  asGroupId,
  asInvitationId,
  asAuthorityId,
  isApplicationStatus,
  isSubscriptionTier,
  isGroupRole,
  isInvitationStatus,
} from "../types";

describe("branded type constructors", () => {
  it("creates an ApplicationUid from a string", () => {
    const uid: ApplicationUid = asApplicationUid("APP-001");
    expect(uid).toBe("APP-001");
  });

  it("creates a WatchZoneId from a string", () => {
    const id: WatchZoneId = asWatchZoneId("zone-123");
    expect(id).toBe("zone-123");
  });

  it("creates a GroupId from a string", () => {
    const id: GroupId = asGroupId("group-abc");
    expect(id).toBe("group-abc");
  });

  it("creates an InvitationId from a string", () => {
    const id: InvitationId = asInvitationId("inv-xyz");
    expect(id).toBe("inv-xyz");
  });

  it("creates an AuthorityId from a number", () => {
    const id: AuthorityId = asAuthorityId(42);
    expect(id).toBe(42);
  });
});

describe("type guards", () => {
  describe("isApplicationStatus", () => {
    it("returns true for valid statuses", () => {
      expect(isApplicationStatus("Undecided")).toBe(true);
      expect(isApplicationStatus("Approved")).toBe(true);
      expect(isApplicationStatus("Refused")).toBe(true);
      expect(isApplicationStatus("Withdrawn")).toBe(true);
      expect(isApplicationStatus("Appealed")).toBe(true);
      expect(isApplicationStatus("Not Available")).toBe(true);
    });

    it("returns false for invalid statuses", () => {
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

  describe("isGroupRole", () => {
    it("returns true for valid roles", () => {
      expect(isGroupRole("Owner")).toBe(true);
      expect(isGroupRole("Member")).toBe(true);
    });

    it("returns false for invalid roles", () => {
      expect(isGroupRole("Admin")).toBe(false);
    });
  });

  describe("isInvitationStatus", () => {
    it("returns true for valid statuses", () => {
      expect(isInvitationStatus("Pending")).toBe(true);
      expect(isInvitationStatus("Accepted")).toBe(true);
      expect(isInvitationStatus("Declined")).toBe(true);
    });

    it("returns false for invalid statuses", () => {
      expect(isInvitationStatus("Expired")).toBe(false);
    });
  });
});
