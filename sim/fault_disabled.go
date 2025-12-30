//go:build !fault_injection

package sim

// TODO: Optionally make the fault injection return a real error message from the standard library.

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
	FaultChanceGeneric float32 = 0

	FaultChancePanic            float32 = 0
	FaultChanceAssertionFailure float32 = 0

	FaultChanceIOGeneric float32 = 0
	FaultChanceIODisk    float32 = 0
	FaultChanceIONetwork float32 = 0

	FaultChanceLatency float32  = 0
	DefaultLatencyMin  Duration = 0
	DefaultLatencyMax  Duration = 0

	FaultChanceMemorySpike float32 = 0
	FaultMemorySpikeBytes  float32 = 0
)

func Panic() {
}

func PanicN(chance float32) {
}

func AssertionFailure() {
}

func AssertionFailureN(chance float32) {
}

func Bool() bool {
}

func BoolN(chance float32) bool {
}

func Err(err *error) error {
	return *err
}

func ErrN(chance float32, err *error) error {
	return *err
}

func IOErr(err *error) error {
	return *err
}

func IOErrN(chance float32, err *error) error {
	return *err
}

func IODiskErr(err *error) error {
	return *err
}

func IODiskErrN(chance float32, err *error) error {
	return *err
}

func IONetworkErr(err *error) error {
	return *err
}

func IONetworkErrN(chance float32, err *error) error {
	return *err
}

func Latency() {
}

func LatencyN(chance float32, lo, hi Duration) {
}

func MemorySpike(release <-chan struct{}) {
}

func MemorySpikeN(chance, n int, release <-chan struct{}) {
}
