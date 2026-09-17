package collector

import (
	"testing"

	"github.com/shirou/gopsutil/v3/net"
)

func TestNetStatsFromDropsIdleInterfaces(t *testing.T) {
	stats := netStatsFrom([]net.IOCountersStat{
		{Name: "en0", BytesSent: 10, BytesRecv: 20},
		{Name: "gif0"},
		{Name: "en1", BytesRecv: 5},
	})
	if len(stats) != 2 || stats[0].Name != "en0" || stats[1].Name != "en1" {
		t.Errorf("got %+v, want en0 and en1", stats)
	}
}
