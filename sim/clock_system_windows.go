//go:build windows

package sim

type SystemTime struct {
	/*
		Reference: https://github.com/tigerbeetle/tigerbeetle/blob/fff8abc12593e72629c95f3dfd3809ba18f4667f/src/time.zig

			pub const TimeOS = struct {
			    /// Hardware and/or software bugs can mean that the monotonic t may regress.
			    /// One example (of many): https://bugzilla.redhat.com/show_bug.cgi?id=448449
			    /// We crash the process for safety if this ever happens, to protect against infinite loops.
			    /// It's better to crash and come back with a valid monotonic t than get stuck forever.
			    monotonic_guard: u64 = 0,
			    ...
	*/
	MonotonicGuard Moment
}

/*
Reference: https://github.com/tigerbeetle/tigerbeetle/blob/main/src/time.zig

	fn monotonic_windows() u64 {
	    assert(is_windows);
	    // Uses QueryPerformanceCounter() on windows due to it being the highest precision timer
	    // available while also accounting for time spent suspended by default:
	    //
	    // https://docs.microsoft.com/en-us/windows/win32/api/realtimeapiset/nf-realtimeapiset-queryunbiasedinterrupttime#remarks

	    // QPF need not be globally cached either as it ends up being a load from read-only memory
	    // mapped to all processed by the kernel called KUSER_SHARED_DATA (See "QpcFrequency")
	    //
	    // https://docs.microsoft.com/en-us/windows-hardware/drivers/ddi/ntddk/ns-ntddk-kuser_shared_data
	    // https://www.geoffchappell.com/studies/windows/km/ntoskrnl/inc/api/ntexapi_x/kuser_shared_data/index.htm
	    const qpc = os.windows.QueryPerformanceCounter();
	    const qpf = os.windows.QueryPerformanceFrequency();

	    // 10Mhz (1 qpc tick every 100ns) is a common QPF on modern systems.
	    // We can optimize towards this by converting to ns via a single multiply.
	    //
	    // https://github.com/microsoft/STL/blob/785143a0c73f030238ef618890fd4d6ae2b3a3a0/stl/inc/chrono#L694-L701
	    const common_qpf = 10_000_000;
	    if (qpf == common_qpf) return qpc * (std.time.ns_per_s / common_qpf);

	    // Convert qpc to nanos using fixed point to avoid expensive extra divs and
	    // overflow.
	    const scale = (std.time.ns_per_s << 32) / qpf;
	    return @as(u64, @truncate((@as(u96, qpc) * scale) >> 32));
	}
*/
func (t *SystemTime) Monotonic() Moment {
	panic("sim.Clock windows is not yet supported")
}

func (t *SystemTime) Realtime() Moment {
	panic("sim.Clock windows is not yet supported")
}

func (t *SystemTime) Advance(duration Duration) {
	panic("sim.Clock windows is not yet supported")
}
