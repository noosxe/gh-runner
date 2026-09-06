import { describe, it, expect } from "vitest";
import { getSuggestedRunnerLabels } from "./labels";

describe("getSuggestedRunnerLabels", () => {
  it("defaults to self-hosted,linux,arm64 when arguments are omitted", () => {
    expect(getSuggestedRunnerLabels()).toBe("self-hosted,linux,arm64");
  });

  it("formats amd64 architecture correctly", () => {
    expect(getSuggestedRunnerLabels("linux", "amd64")).toBe("self-hosted,linux,amd64");
  });

  it("handles case insensitivity and whitespace", () => {
    expect(getSuggestedRunnerLabels("  LINUX  ", "  AMD64 ")).toBe("self-hosted,linux,amd64");
  });

  it("preserves arm64 when running on arm64", () => {
    expect(getSuggestedRunnerLabels("linux", "arm64")).toBe("self-hosted,linux,arm64");
  });
});
