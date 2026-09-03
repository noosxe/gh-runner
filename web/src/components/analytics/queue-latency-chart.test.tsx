import { describe, it, expect, vi } from "vitest";
import { render, screen, fireEvent } from "@testing-library/react";
import { QueueLatencyChart } from "./queue-latency-chart";
import type { LatencyBucket } from "../../gen/api_pb";

const mockTrend: LatencyBucket[] = [
  {
    timestamp: "2026-09-04T00:00:00Z",
    avgQueueSeconds: 2.1,
    avgRuntimeSeconds: 120.0,
    totalJobs: 15,
    successfulJobs: 14,
    failedJobs: 1,
  } as LatencyBucket,
  {
    timestamp: "2026-09-04T01:00:00Z",
    avgQueueSeconds: 6.5,
    avgRuntimeSeconds: 95.0,
    totalJobs: 22,
    successfulJobs: 21,
    failedJobs: 1,
  } as LatencyBucket,
  {
    timestamp: "2026-09-04T02:00:00Z",
    avgQueueSeconds: 3.8,
    avgRuntimeSeconds: 140.0,
    totalJobs: 10,
    successfulJobs: 10,
    failedJobs: 0,
  } as LatencyBucket,
];

describe("QueueLatencyChart", () => {
  it("renders empty state placeholder when trend has no points", () => {
    render(
      <QueueLatencyChart
        trend={[]}
        averageQueueSeconds={0}
        timeframeHours={24}
        onTimeframeChange={vi.fn()}
      />,
    );

    expect(screen.getByText("No queue latency data yet")).toBeInTheDocument();
  });

  it("renders SVG trend chart, capacity badge, and timeframe selector", () => {
    const onTimeframeMock = vi.fn();
    render(
      <QueueLatencyChart
        trend={mockTrend}
        averageQueueSeconds={4.1}
        timeframeHours={24}
        onTimeframeChange={onTimeframeMock}
      />,
    );

    expect(screen.getByText("Queue Wait-Time Latency")).toBeInTheDocument();
    expect(screen.getByText("Optimal Capacity")).toBeInTheDocument();

    const sevenDaysBtn = screen.getByRole("button", { name: "7d" });
    fireEvent.click(sevenDaysBtn);
    expect(onTimeframeMock).toHaveBeenCalledWith(168);
  });
});
