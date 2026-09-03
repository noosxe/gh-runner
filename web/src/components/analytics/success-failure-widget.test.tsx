import { describe, it, expect } from "vitest";
import { render, screen } from "@testing-library/react";
import { SuccessFailureWidget } from "./success-failure-widget";

describe("SuccessFailureWidget", () => {
  it("renders success rate, counts, and runtime duration", () => {
    render(
      <SuccessFailureWidget
        totalJobs={100}
        successfulJobs={98}
        failedJobs={2}
        successRatePercent={98.0}
        averageRuntimeSeconds={185}
      />,
    );

    expect(screen.getByText("98.0%")).toBeInTheDocument();
    expect(screen.getByText("100 total runs")).toBeInTheDocument();
    expect(screen.getByText("98")).toBeInTheDocument();
    expect(screen.getByText("2")).toBeInTheDocument();
    expect(screen.getByText("3m 5s")).toBeInTheDocument();
  });
});
