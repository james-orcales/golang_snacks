package sim

import (
	"math/rand/v2"
	"sync"
	stdtime "time"

	"github.com/james-orcales/golang_snacks/invariant"
)

// TODO: Have a default seed based on the current git commit hash.

const (
	Nanosecond  = 1
	Microsecond = Nanosecond * 1000
	Millisecond = Microsecond * 1000
	Second      = Millisecond * 1000
	Minute      = Second * 60
	Hour        = Minute * 60
	Day         = Hour * 24
	Week        = Day * 7
)

// Moment describes nanoseconds elapsed since an arbitrary, clock-specific start. Only
// differences between two `Moment`s produced by the same clock are meaningful, yielding a
// `Duration`.
//
// Unit: Nanosecond
type Moment int64
type Duration int64

func (moment Moment) Delta(duration Duration) Moment {
	result := moment + Moment(duration)
	invariant.Sometimes(result < 0, "Moment.Delta is negative")
	invariant.Sometimes(result >= 0, "Moment.Delta is non-negative")
	return result
}

func (moment Moment) Advance(duration Duration) Moment {
	invariant.Always(duration >= 0, "Moment.Advance duration is non-negative")
	result := moment + Moment(duration)
	invariant.Always(result >= moment, "Moment.Advance moves forward in time")
	return result
}

func (moment Moment) Since(earlier Moment) Duration {
	invariant.Always(earlier <= moment, "Moment.Since argument is earlier than receiver")
	return Duration(moment - earlier)
}

// WARN: Moment objects are not guaranteed to be nanoseconds elapsed since the UNIX epoch. This is
// simply a convenience function. Consider Monotonic moments describing system uptime versus
// Realtime moments describing nanoseconds since UNIX epoch.
func (moment Moment) StdTime() stdtime.Time {
	return stdtime.Unix(0, int64(moment))
}

type Time interface {
	/*
		NOTE: Advance() only works with virtual clocks. System clocks should noop.

		With system clocks, time progresses implicitly. In virtual clocks, we must manually
		progress time. This method takes in an inclusive Duration range. Rarely do we want
		to have a fixed Duration to progress time with. Take for example a logger. Each log
		write has varying amounts of CPU+IO execution time depending on the context that it
		carries. For example,

			// This takes ~20ns
			logger.Clone().Info().Msg("my log message")
			// This takes ~120ns
			logger.Clone().Info().Str("key", "value").<10 more fields>.Msg("my log message")

		We can embed sim.UniversalTime.Advance inside itlog.Logger.Msg to simulate execution time.

			func (lgr *Logger) Msg(msg string) {
				sim.UniversalTime.Advance(20, 120)
			}

		Scaling CPU time directly with some variable is a great alternative if the struct
		were to track those metrics.

			func (lgr *Logger) Msg(msg string) {
				n := lgr.FieldCount
				overhead := n * 10
				sim.UniversalTime.Advance(n, n)
			}

		In complex functions, only account for the execution time of the inlined operations. Do not
		take into account the execution time of function calls as those may already call Advance(),
		essentially doubling the predicted execution time.

		func query() {
			// 50ns-30s depending on cache validation, network latency, or server errors
			user := getUser()


			// Assume this computation always costs 100-200ns
			var result any
			for key, val := range user.PrivateDataThatTheyDidNotConsentToProviding {
				// something fairly complex
			}
			sim.UniversalTime.Advance(100, 200)

			// another 50ns-30s
			submitResult(result)
		}

		The easiest would be to just have one Advance() to represent the execution time of
		the current function. If you're confident in your prediction, you can even break it
		up into multiple calls, specifying the execution time of each exact section. (No
		examples for this)

		NOTE:

		1) This requires predicted CPU time to be consistent across all packages. Someone
		should probably be dedicated to hardcoding the values so that their system spec will
		be the baseline. Or serialize `go bench` results and then everyone can compare their
		machine's performance to that. Or naively ignore CPU time and just focus on IO time.
		(refer to Note #2)

		2) You don't need to go crazy with Advance. Just like the invariant package, aim for
		incremental integration with your codebase. A good starting point is simply with IO
		operations which have macro-level timing, usually in seconds to minutes.
	*/
	Advance(lo Duration, hi Duration)

	// Equivalent to stdtime.Sleep()
	Sleep(Duration)

	// Nanoseconds elapsed since system startup. Includes time during system suspension.
	// Virtual clocks should Advance by some amount with each call to simulate syscall overhead.
	Monotonic() Moment

	// Nanoseconds elapsed since UNIX epoch. Equivalent to stdtime.Now().UnixNano()
	// Virtual clocks should Advance by some amount with each call to simulate syscall overhead.
	Realtime() Moment
}

// All packages should have a global Time set to this by default. This enables simulation testing
// for the whole runtime.
//
// Usage:
//
//	package foo
//
//	import "github.com/james-orcales/golang_snacks/sim"
//
//	var Time = sim.UniversalTime
var UniversalTime Time = &SystemTime{}

func Advance(lo, hi Duration) {
	UniversalTime.Advance(lo, hi)
}

func Sleep(duration Duration) {
	UniversalTime.Sleep(duration)
}

func Monotonic() Moment {
	return UniversalTime.Monotonic()
}

func Realtime() Moment {
	return UniversalTime.Realtime()
}

// TODO: There's more variables to account for to achieve extreme realism such as drift, slew and
// NTP polling backoff. The value of that is TBD however.
type VirtualTime struct {
	Mutex sync.Mutex

	// Time is the actual "universe" time of the simulation. This is essentially just monotonic
	// time without the resolution.
	Time Moment

	// Epoch represents the reference moments captured upon process initialization.
	EpochTime     Moment // immutable
	EpochRealtime Moment // immutable

	// Overhead is the number of nanoseconds elapsed when calling Monotonic() and Realtime()
	// to simulate syscall overhead.
	Overhead Duration // immutable

	// Time resolutions describe the smallest discrete increment that a physical clock can measure.
	// Real-world hardware clocks, such as quartz crystals, tick at fixed intervals.
	//
	// MonotonicResolution ensures that MonotonicCurrent only advances in multiples of this tick,
	// accurately representing the granularity of the underlying hardware. For example, if
	// MonotonicResolution = 100ns and Advance(30,30), a sequence of Time:Monotonic()
	// updates would be:
	// 0:0 -> 30:0 -> 60:0 -> 90:0 -> 120:100 -> 150:100 -> 180:100 -> 210:200.
	MonotonicResolution Duration // immutable
	RealtimeResolution  Duration // immutable

	// Interval after which the realtime clock is corrected, simulating NTP polling.
	// Should be seconds in powers of 2 as per the NTP standard (e.g. 64 * Second, 128 * Second)
	// Reference: https://www.ntp.org/documentation/4.2.8-series/poll/
	NTPInterval Duration // immutable
	NTPNext     Moment   // Monotonic

	// Jump simulates the clock jumping backwards or forwards. This can occur during NTP syncs
	// or manual system time modification. This ensures that the consumer does not rely on
	// Realtime moments to compute durations or determine event order. It has a chance to
	// occur with every call to all Time methods.
	Jump        Duration
	JumpStepMin Duration // immutable, inclusive
	JumpStepMax Duration // immutable, inclusive
	JumpChance  float32  // immutable
}

var mysteryTimestamp = Moment(stdtime.Date(2020, stdtime.April, 9, 16, 15, 0, 0, stdtime.UTC).UnixNano())

func NewVirtualTime(vtime *VirtualTime) *VirtualTime {
	if vtime == nil {
		vtime = &VirtualTime{
			EpochTime:           1 * Minute,
			EpochRealtime:       mysteryTimestamp,
			Overhead:            20 * Nanosecond,
			MonotonicResolution: 100 * Nanosecond,
			RealtimeResolution:  1 * Microsecond,
			NTPInterval:         64 * Second,
			JumpStepMin:         128 * Millisecond,
			JumpStepMax:         1 * Day,
			JumpChance:          0.01,
		}
	}

	invariant.Sometimes(vtime.EpochTime < vtime.EpochRealtime, "Initial EpochRealtime is before Unix epoch")
	vtime.Time = vtime.EpochTime
	invariant.Ensure(vtime.Overhead > 0, "VirtualTime.Overhead is a positive integer")
	invariant.Ensure(vtime.MonotonicResolution > 0, "VirtualTime.MonotonicResolution is a positive integer")
	invariant.Ensure(vtime.RealtimeResolution > 0, "VirtualTime.RealtimeResolution is a positive integer")
	invariant.Ensure(vtime.NTPInterval > 0, "VirtualTime.NTPInterval is a positive integer")
	vtime.NTPNext = vtime.Time

	vtime.Jump = 0
	invariant.Ensure(vtime.JumpStepMin > 0, "VirtualTime.JumpStepMin is a positive integer")
	invariant.Ensure(vtime.JumpStepMax >= vtime.JumpStepMin, "VirtualTime.JumpStepMax is greater than or equal to JumpStepMin")
	invariant.Ensure(vtime.JumpChance >= 0 && vtime.JumpChance <= 1.0, "VirtualTime.JumpChance is 0.0-1.0")

	return vtime
}

func (vtime *VirtualTime) Advance(lo, hi Duration) {
	invariant.Always(lo <= hi, "VirtualTime.Advance lo <= hi")
	jumpStep := vtime.randJump()

	step := lo
	if lo != hi {
		step = lo + Duration(rand.Int64N(int64(hi-lo+1)))
	}

	vtime.Mutex.Lock()
	vtime.Time = vtime.Time.Advance(step)
	if shouldSync := vtime.Time.Since(vtime.NTPNext) <= 0; shouldSync {
		vtime.NTPNext = vtime.NTPNext.Advance(vtime.NTPInterval)
		vtime.Jump = 0
	} else {
		vtime.Jump += jumpStep
	}
	vtime.Mutex.Unlock()
}

func (vtime *VirtualTime) Sleep(duration Duration) {
	invariant.Always(duration >= 0, "VirtualTime.Sleep argument is a non-negative integer")
	// This is hardcoded for simplicity. stdtime.Sleep() is inherently inaccurate.
	vtime.Advance(duration+(100*Microsecond), duration+(1*Millisecond))
}

func (vtime *VirtualTime) Monotonic() (now Moment) {
	jumpStep := vtime.randJump()

	vtime.Mutex.Lock()
	vtime.Time = vtime.Time.Advance(vtime.Overhead)
	now = vtime.Time - vtime.Time%Moment(vtime.MonotonicResolution)
	if shouldSync := now.Since(vtime.NTPNext) <= 0; shouldSync {
		vtime.NTPNext = vtime.NTPNext.Advance(vtime.NTPInterval)
		vtime.Jump = 0
	} else {
		vtime.Jump += jumpStep
	}
	vtime.Mutex.Unlock()
	return now
}

func (vtime *VirtualTime) Realtime() (now Moment) {
	jumpStep := vtime.randJump()

	vtime.Mutex.Lock()
	vtime.Time = vtime.Time.Advance(vtime.Overhead)
	now = vtime.Time - vtime.Time%Moment(vtime.RealtimeResolution)
	if shouldSync := now.Since(vtime.NTPNext) <= 0; shouldSync {
		vtime.NTPNext = vtime.NTPNext.Advance(vtime.NTPInterval)
		vtime.Jump = 0
	} else {
		vtime.Jump += jumpStep
		now = now.Delta(vtime.Jump)
	}
	vtime.Mutex.Unlock()
	return now
}

func (vtime *VirtualTime) randJump() Duration {
	var jumpStep Duration
	if rand.Float32() < vtime.JumpChance {
		jumpStep = vtime.JumpStepMin + Duration(rand.Int64N(int64(vtime.JumpStepMax)+1-int64(vtime.JumpStepMin)))
		if rand.Float32() >= 0.5 {
			jumpStep *= -1
		}
	}
	return jumpStep
}
