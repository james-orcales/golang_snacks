//go:build darwin

package sim

/*
#include <mach/mach_time.h>
*/
import "C"
import (
	"sync"
	"time"
	stdtime "time"

	"github.com/james-orcales/golang_snacks/invariant"
)

func init() {
	code := C.mach_timebase_info(&timebase)
	if code != 0 {
		panic("mach_timebase_info failed")
	}
}

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

var (
	timebase C.mach_timebase_info_data_t
	once     sync.Once
)

/*
Reference: https://github.com/tigerbeetle/tigerbeetle/blob/fff8abc12593e72629c95f3dfd3809ba18f4667f/src/time.zig

	fn monotonic_darwin() u64 {
	    assert(is_darwin);
	    // Uses mach_continuous_time() instead of mach_absolute_time() as it counts while suspended.
	    //
	    // https://developer.apple.com/documentation/kernel/1646199-mach_continuous_time
	    // https://opensource.apple.com/source/Libc/Libc-1158.1.2/gen/t_gettime.c.auto.html
	    const darwin = struct {
	        const mach_timebase_info_t = system.mach_timebase_info_data;
	        extern "c" fn mach_timebase_info(info: *mach_timebase_info_t) system.kern_return_t;
	        extern "c" fn mach_continuous_time() u64;
	    };

	    // mach_timebase_info() called through libc already does global caching for us
	    //
	    // https://opensource.apple.com/source/xnu/xnu-7195.81.3/libsyscall/wrappers/mach_timebase_info.c.auto.html
	    var info: darwin.mach_timebase_info_t = undefined;
	    if (darwin.mach_timebase_info(&info) != 0) @panic("mach_timebase_info() failed");

	    const now = darwin.mach_continuous_time();
	    return (now * info.numer) / info.denom;
	}
*/
func (stime *SystemTime) Monotonic() Moment {
	ticks := C.mach_continuous_time()
	ns := Moment((uint64(ticks) * uint64(timebase.numer)) / uint64(timebase.denom))
	if ns < stime.MonotonicGuard {
		panic("a hardware/kernel bug regressed the hardware t")
	}
	stime.MonotonicGuard = ns
	return ns
}

func (stime *SystemTime) Realtime() Moment {
	return Moment(stdtime.Now().UnixNano())
}

func (stime *SystemTime) Sleep(duration Duration) {
	invariant.Always(duration >= 0, "sleep duration must be a non-negative integer")
	stdtime.Sleep(time.Duration(duration))
}

func (stime *SystemTime) Advance(lo Duration, hi Duration) {
	// noop
}
