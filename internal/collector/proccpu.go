package collector

/*
#include <libproc.h>
#include <sys/proc_info.h>
#include <mach/mach_time.h>
#include <string.h>

// Cumulative user+system CPU time of pid in nanoseconds, or -1 if unavailable
// (typically processes owned by other users).
static long long mm_proc_cpu_ns(int pid) {
	struct proc_taskinfo ti;
	if (proc_pidinfo(pid, PROC_PIDTASKINFO, 0, &ti, sizeof(ti)) != sizeof(ti)) {
		return -1;
	}
	static mach_timebase_info_data_t tb;
	if (tb.denom == 0) mach_timebase_info(&tb);
	return (long long)((ti.pti_total_user + ti.pti_total_system) * tb.numer / tb.denom);
}

// Executable file name of pid (not truncated like p_comm).
static int mm_proc_name(int pid, char *buf, int len) {
	char path[PROC_PIDPATHINFO_MAXSIZE];
	if (proc_pidpath(pid, path, sizeof(path)) <= 0) return -1;
	char *base = strrchr(path, '/');
	base = base ? base + 1 : path;
	strlcpy(buf, base, len);
	return 0;
}
*/
import "C"

import (
	"time"
	"unsafe"

	"golang.org/x/sys/unix"
)

// ProcessCPUTimes returns cumulative CPU time for every process whose stats
// are readable. It makes one sysctl plus one proc_pidinfo call per process and
// no name lookups, so it is cheap enough to run periodically in the background.
func ProcessCPUTimes() (map[int32]time.Duration, error) {
	procs, err := unix.SysctlKinfoProcSlice("kern.proc.all")
	if err != nil {
		return nil, err
	}
	times := make(map[int32]time.Duration, len(procs))
	for i := range procs {
		pid := procs[i].Proc.P_pid
		if ns := C.mm_proc_cpu_ns(C.int(pid)); ns >= 0 {
			times[pid] = time.Duration(ns)
		}
	}
	return times, nil
}

// ProcessName returns the executable name of pid, or "" if unavailable.
func ProcessName(pid int32) string {
	var buf [256]C.char
	if C.mm_proc_name(C.int(pid), &buf[0], C.int(len(buf))) != 0 {
		return ""
	}
	return C.GoString((*C.char)(unsafe.Pointer(&buf[0])))
}
