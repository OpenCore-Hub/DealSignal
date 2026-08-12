import { describe, expect, it } from "vitest";
import { resolveShareViewerDomains } from "./viewerDomains";

describe("resolveShareViewerDomains", () => {
  it("returns empty when no domain configured", () => {
    expect(resolveShareViewerDomains(null)).toEqual({
      availableDomains: [],
      pendingHostname: "",
    });
    expect(resolveShareViewerDomains({ hostname: "", status: "", cnameHost: "", cnameTarget: "" })).toEqual({
      availableDomains: [],
      pendingHostname: "",
    });
  });

  it("exposes verified Brand hostname in the share dropdown", () => {
    expect(
      resolveShareViewerDomains({
        hostname: "invest.example.com",
        status: "verified",
        cnameHost: "invest.example.com",
        cnameTarget: "cname.dealsignal.com",
      }),
    ).toEqual({
      availableDomains: ["invest.example.com"],
      pendingHostname: "",
    });
  });

  it("keeps pending Brand hostname out of selectable options", () => {
    expect(
      resolveShareViewerDomains({
        hostname: "invest.example.com",
        status: "pending",
        cnameHost: "invest.example.com",
        cnameTarget: "cname.dealsignal.com",
      }),
    ).toEqual({
      availableDomains: [],
      pendingHostname: "invest.example.com",
    });
  });
});
