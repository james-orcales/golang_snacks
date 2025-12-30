//go:build fault_injection

package sim

import (
	"errors"
	"math/rand/v2"

	"github.com/james-orcales/golang_snacks/invariant"
)

// TODO: Optionally make the fault injection return a real error message from the standard library.
// TODO: Create helper functions that spawn goroutines creating latency/memoryspikes in the background.

const (
	FaultErrorPrefix = "Fault Injected: "

	Kilobyte = 1 * 1000
	Megabyte = Kilobyte * 1000
	Gigabyte = Megabyte * 1000
	Terabyte = Gigabyte * 1000

	Kibibyte = 1 * 1024
	Mebibyte = Kibibyte * 1024
	Gibibyte = Mebibyte * 1024
	Tebibyte = Gibibyte * 1024
)

// Fault probabilities are grouped by subsystems/devices.
// This allows targeted fault injection to model cascading or correlated failures.
// Example: progressively increase disk IO faults to emulate a degrading SSD
// while leaving other IO paths mostly healthy.
var (
	FaultChanceGeneric float32 = 0.10

	FaultChancePanic            float32 = 0.10
	FaultChanceAssertionFailure float32 = 0.10

	FaultChanceIOGeneric float32 = 0.10
	FaultChanceIODisk    float32 = 0.10
	FaultChanceIONetwork float32 = 0.10

	FaultChanceLatency float32  = 0.10
	DefaultLatencyMin  Duration = 100 * Microsecond
	DefaultLatencyMax  Duration = 5 * Second

	FaultChanceMemorySpike float32 = 0.05
	FaultMemorySpikeBytes  int     = 50 * Megabyte
)

func Panic() {
	PanicN(FaultChancePanic)
}

func PanicN(chance float32) {
	if rand.Float32() < chance {
		panic(FaultErrorPrefix + "Panic")
	}
}

func AssertionFailure() {
	AssertionFailureN(FaultChanceAssertionFailure)
}

func AssertionFailureN(chance float32) {
	if !invariant.AssertionFailureIsFatal && rand.Float32() < chance {
		invariant.Ensure(false, "Fault Injected")
	}
}

func Bool() bool {
	return BoolN(FaultChanceGeneric)
}

func BoolN(chance float32) bool {
	return rand.Float32() < chance
}

// Err randomly returns an error based on FaultChanceGeneric.
//
// Usage:
//
//	err := fallible()
//	if sim.Err(&err) != nil {
//		handleError(err)
//	}
//
//	if _, ok := anotherFallible(); !ok {
//		err = errors.New("whatever")
//	}
//	if sim.Err(&err) != nil {
//		return
//	}
func Err(err *error) error {
	return ErrN(FaultChanceGeneric, err)
}

func ErrN(chance float32, err *error) error {
	if err == nil && rand.Float32() < chance {
		*err = errors.New(FaultErrorPrefix + "Generic error")
	}
	return *err
}

func IOErr(err *error) error {
	return IOErrN(FaultChanceGeneric, err)
}

func IOErrN(chance float32, err *error) error {
	if err == nil && rand.Float32() < chance {
		*err = errors.New(FaultErrorPrefix + "IO error (Generic)")
	}
	return *err
}

func IODiskErr(err *error) error {
	return IODiskErrN(FaultChanceIODisk, err)
}

func IODiskErrN(chance float32, err *error) error {
	if err == nil && rand.Float32() < chance {
		*err = errors.New(FaultErrorPrefix + "IO error (Disk)")
	}
	return *err
}

// TODO: Add latency
func IONetworkErr(err *error) error {
	return IONetworkErrN(FaultChanceIONetwork, err)
}

func IONetworkErrN(chance float32, err *error) error {
	if err == nil && rand.Float32() < chance {
		*err = errors.New(FaultErrorPrefix + "IO error (Network)")
	}
	return *err
}

func Latency() {
	LatencyN(FaultChanceLatency, DefaultLatencyMin, DefaultLatencyMax)
}

func LatencyN(chance float32, lo, hi Duration) {
	if rand.Float32() < chance {
		UniversalTime.Advance(lo, hi)
	}
}

func MemorySpike(release <-chan struct{}) {
	MemorySpikeN(FaultChanceMemorySpike, FaultMemorySpikeBytes, release)
}

// Usage:
//
//	release := make(chan struct{})
//	go simulation.MemorySpike(50*1024*1024, release) // 50 MB spike
//	// do work under memory pressure
//	close(release) // release the memory
//
// TODO: Verify if this gets optimized away
func MemorySpikeN(chance float32, n int, release <-chan struct{}) {
	if rand.Float32() < chance {
		garbage := make([]byte, n)
		<-release
		garbage[0] = 42
	}
}
