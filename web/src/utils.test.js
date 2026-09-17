import {describe, it, expect} from "vitest";
import {fmtBytes, netRates, diskIORates, primaryIface, primaryDisk, toPoints, timeTicks, fmtTick} from "./utils.js";

describe("fmtBytes", () => {
  it("formats raw bytes", () => expect(fmtBytes(500)).toBe("500 B/s"));
  it("formats kilobytes", () => expect(fmtBytes(1500)).toBe("1.5 KB/s"));
  it("formats megabytes", () => expect(fmtBytes(2_500_000)).toBe("2.5 MB/s"));
  it("formats gigabytes", () => expect(fmtBytes(1_500_000_000)).toBe("1.5 GB/s"));
});

describe("netRates", () => {
  const snap = (ts, bytesRecv, bytesSent) => ({
    ts,
    net_stats: [{name: "en0", bytes_recv: bytesRecv, bytes_sent: bytesSent}],
  });

  it("returns empty array for single-entry history", () => {
    expect(netRates([snap(0, 0, 0)], "en0")).toEqual([]);
  });

  it("computes rates correctly over one second", () => {
    const history = [snap(0, 0, 0), snap(1, 1000, 500)];
    const [rate] = netRates(history, "en0");
    expect(rate.in).toBe(1000);
    expect(rate.out).toBe(500);
  });

  it("divides by elapsed seconds", () => {
    const history = [snap(0, 0, 0), snap(2, 2000, 1000)];
    const [rate] = netRates(history, "en0");
    expect(rate.in).toBe(1000);
    expect(rate.out).toBe(500);
  });

  it("clamps counter resets to zero", () => {
    const history = [snap(0, 1000, 500), snap(1, 0, 0)];
    const [rate] = netRates(history, "en0");
    expect(rate.in).toBe(0);
    expect(rate.out).toBe(0);
  });

  it("returns zeros for missing interface", () => {
    const history = [{ts: 0, net_stats: []}, {ts: 1, net_stats: []}];
    const [rate] = netRates(history, "en0");
    expect(rate).toEqual({in: 0, out: 0});
  });

  it("skips zero-length time intervals", () => {
    const history = [snap(1, 0, 0), snap(1, 1000, 500)];
    const [rate] = netRates(history, "en0");
    expect(rate).toEqual({in: 0, out: 0});
  });
});

describe("diskIORates", () => {
  const snap = (ts, read, write) => ({
    ts,
    disk_io_stats: [{name: "disk0", read_bytes: read, write_bytes: write}],
  });

  it("computes rates correctly", () => {
    const history = [snap(0, 0, 0), snap(2, 2000, 1000)];
    const [rate] = diskIORates(history, "disk0");
    expect(rate.read).toBe(1000);
    expect(rate.write).toBe(500);
  });

  it("clamps counter resets to zero", () => {
    const history = [snap(0, 5000, 5000), snap(1, 0, 0)];
    const [rate] = diskIORates(history, "disk0");
    expect(rate.read).toBe(0);
    expect(rate.write).toBe(0);
  });
});

describe("primaryIface", () => {
  it("returns null for null snap", () => expect(primaryIface(null)).toBeNull());
  it("returns null for empty net_stats", () => expect(primaryIface({net_stats: []})).toBeNull());

  it("prefers en0 over busier interfaces", () => {
    const snap = {net_stats: [
      {name: "lo0",  bytes_recv: 9999, bytes_sent: 9999},
      {name: "en0",  bytes_recv: 100,  bytes_sent: 100},
      {name: "en1",  bytes_recv: 200,  bytes_sent: 200},
    ]};
    expect(primaryIface(snap)).toBe("en0");
  });

  it("falls back to busiest non-loopback interface", () => {
    const snap = {net_stats: [
      {name: "lo0",  bytes_recv: 9999, bytes_sent: 9999},
      {name: "eth0", bytes_recv: 100,  bytes_sent: 50},
      {name: "eth1", bytes_recv: 500,  bytes_sent: 50},
    ]};
    expect(primaryIface(snap)).toBe("eth1");
  });

  it("excludes utun interfaces", () => {
    const snap = {net_stats: [
      {name: "utun0", bytes_recv: 9999, bytes_sent: 9999},
      {name: "eth0",  bytes_recv: 100,  bytes_sent: 50},
    ]};
    expect(primaryIface(snap)).toBe("eth0");
  });

  it("returns null when only loopback and tunnel interfaces are present", () => {
    const snap = {net_stats: [
      {name: "lo0",   bytes_recv: 100, bytes_sent: 100},
      {name: "utun0", bytes_recv: 200, bytes_sent: 200},
    ]};
    expect(primaryIface(snap)).toBeNull();
  });
});

describe("primaryDisk", () => {
  it("returns null for null snap", () => expect(primaryDisk(null)).toBeNull());
  it("returns null for empty disk_io_stats", () => expect(primaryDisk({disk_io_stats: []})).toBeNull());

  it("returns lexicographically first disk name", () => {
    const snap = {disk_io_stats: [{name: "disk1"}, {name: "disk0"}, {name: "disk2"}]};
    expect(primaryDisk(snap)).toBe("disk0");
  });
});

describe("gaps", () => {
  it("toPoints converts to ms and breaks the line across gaps", () => {
    expect(toPoints([0, 5, 100, 105], [1, 2, 3, 4], 15)).toEqual([
      {x: 0, y: 1},
      {x: 5000, y: 2},
      {x: 52500, y: null},
      {x: 100000, y: 3},
      {x: 105000, y: 4},
    ]);
  });

  it("netRates returns null for rates spanning a gap", () => {
    const snap = (ts, rx) => ({ts, net_stats: [{name: "en0", bytes_recv: rx, bytes_sent: 0}]});
    const rates = netRates([snap(0, 0), snap(5, 500), snap(3600, 1000)], "en0", 15);
    expect(rates[0].in).toBe(100);
    expect(rates[1]).toEqual({in: null, out: null});
  });

  it("diskIORates returns null for rates spanning a gap", () => {
    const snap = (ts, r) => ({ts, disk_io_stats: [{name: "disk0", read_bytes: r, write_bytes: 0}]});
    const rates = diskIORates([snap(0, 0), snap(3600, 1000)], "disk0", 15);
    expect(rates[0]).toEqual({read: null, write: null});
  });
});

describe("timeTicks", () => {
  it("picks round steps and stays within range", () => {
    const min = new Date(2026, 0, 1, 10, 7).getTime();
    const max = min + 3600_000;
    const ticks = timeTicks(min, max, 8);
    expect(ticks.length).toBeGreaterThan(1);
    expect(ticks.length).toBeLessThanOrEqual(8);
    expect(ticks[0]).toBeGreaterThanOrEqual(min);
    expect(ticks.at(-1)).toBeLessThanOrEqual(max);
    expect(new Date(ticks[0]).getMinutes() % 10).toBe(0);
  });

  it("aligns daily ticks to local midnight", () => {
    const min = new Date(2026, 0, 1, 10).getTime();
    const ticks = timeTicks(min, min + 30 * 86400_000, 8);
    for (const t of ticks) {
      expect(new Date(t).getHours()).toBe(0);
    }
  });
});

describe("fmtTick", () => {
  it("shows seconds only for sub-minute steps", () => {
    const t = new Date(2026, 0, 1, 9, 5, 30).getTime();
    expect(fmtTick(t, 10_000)).toBe("09:05:30");
    expect(fmtTick(t, 300_000)).toBe("09:05");
  });
});
