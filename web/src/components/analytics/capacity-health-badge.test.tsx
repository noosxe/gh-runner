import { describe, it, expect } from "vitest";
import { render, screen } from "@testing-library/react";
import { CapacityHealthBadge, getCapacityStatus } from "./capacity-health-badge";

describe("CapacityHealthBadge & getCapacityStatus", () => {
  it("evaluates optimal capacity for sub-5s latency", () => {
    const status = getCapacityStatus(2.4);
    expect(status.status).toBe("optimal");
    expect(status.label).toBe("Optimal Capacity");

    render(<CapacityHealthBadge avgQueueSeconds={2.4} />);
    expect(screen.getByText("Optimal Capacity")).toBeInTheDocument();
  });

  it("evaluates moderate capacity for 5s-30s latency", () => {
    const status = getCapacityStatus(12.5);
    expect(status.status).toBe("moderate");
    expect(status.label).toBe("Moderate Load");

    render(<CapacityHealthBadge avgQueueSeconds={12.5} />);
    expect(screen.getByText("Moderate Load")).toBeInTheDocument();
  });

  it("evaluates constrained capacity for >30s latency", () => {
    const status = getCapacityStatus(45.0);
    expect(status.status).toBe("constrained");
    expect(status.label).toBe("Capacity Constrained");

    render(<CapacityHealthBadge avgQueueSeconds={45.0} />);
    expect(screen.getByText("Capacity Constrained")).toBeInTheDocument();
  });
});
