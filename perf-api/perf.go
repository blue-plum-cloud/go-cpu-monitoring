package perf

/*
#cgo CFLAGS: -I.
#include <stdio.h>
#include <stdlib.h>
#include "perf_api.h"
// #include "perf_api.c"

*/
import "C"
import (
	_ "fmt"
	"log"
	"unsafe"
)

func UNUSED(x ...interface{}) {}

type PerfType C.uint64_t

const (
	// type
	PERF_TYPE_HARDWARE PerfType = C.PERF_TYPE_HARDWARE
	PERF_TYPE_HW_CACHE PerfType = C.PERF_TYPE_HW_CACHE
	PERF_TYPE_RAW      PerfType = C.PERF_TYPE_RAW

	// config, PERF_TYPE_HARDWARE
	PERF_COUNT_HW_CPU_CYCLES     PerfType = C.PERF_COUNT_HW_CPU_CYCLES
	PERF_COUNT_HW_INSTRUCTIONS   PerfType = C.PERF_COUNT_HW_INSTRUCTIONS
	PERF_COUNT_HW_REF_CPU_CYCLES PerfType = C.PERF_COUNT_HW_REF_CPU_CYCLES

	PERF_COUNT_HW_CACHE_L1D           PerfType = C.PERF_COUNT_HW_CACHE_L1D
	PERF_COUNT_HW_CACHE_OP_READ       PerfType = C.PERF_COUNT_HW_CACHE_OP_READ
	PERF_COUNT_HW_CACHE_RESULT_ACCESS PerfType = C.PERF_COUNT_HW_CACHE_RESULT_ACCESS
)

// PerfEventConfig holds the configuration for performance events
type PerfEventConfig struct {
	pe      C.struct_perf_event_attr
	Fds     []C.int
	Ids     []C.uint64_t
	Types   []C.uint64_t
	Configs []C.uint64_t
}

// NewPerfEventConfig initializes a new PerfEventConfig with the specified number of events
func NewPerfEventConfig(numEvents int) PerfEventConfig {
	return PerfEventConfig{
		Fds:     make([]C.int, numEvents),
		Ids:     make([]C.uint64_t, numEvents),
		Types:   make([]C.uint64_t, numEvents),
		Configs: make([]C.uint64_t, numEvents),
	}
}

func ConfigPerf(pec *PerfEventConfig, cpu int) int {
	group_fd := C.config_perf_multi(
		(*C.struct_perf_event_attr)(unsafe.Pointer(&pec.pe)), // Assuming `pe` is correctly initialized
		&pec.Fds[0],
		&pec.Ids[0],
		&pec.Types[0],
		&pec.Configs[0],
		(C.int)(len(pec.Fds)),
		(C.int)(cpu),
	)
	return int(group_fd)
}

func SetupPerf(mode bool) (PerfEventConfig, []string) {
	var strings []string
	var types []PerfType
	var configs []PerfType
	if mode {
		strings = []string{"Instructions",
			"Cycles",
			"All Retired Memory Instructions",
			"L1D Misses",
			"L1D Hits",
		}

		types = []PerfType{PERF_TYPE_HARDWARE,
			PERF_TYPE_HARDWARE,
			PERF_TYPE_RAW,
			PERF_TYPE_RAW,
			PERF_TYPE_RAW,
		}

		configs = []PerfType{PERF_COUNT_HW_INSTRUCTIONS,
			PERF_COUNT_HW_REF_CPU_CYCLES,
			0x83D0,
			0x08D1,
			0x01D1,
		}
	} else {
		strings = []string{"Instructions",
			"L2 Misses",
			"L2 Hits",
			"L3 Misses",
			"L3 Hits",
		}

		types = []PerfType{PERF_TYPE_HARDWARE,
			PERF_TYPE_RAW,
			PERF_TYPE_RAW,
			PERF_TYPE_RAW,
			PERF_TYPE_RAW,
		}

		configs = []PerfType{PERF_COUNT_HW_INSTRUCTIONS,
			0x10D1,
			0x02D1,
			0x20D1,
			0x04D1,
		}
	}

	hw_cache_op_ids := []PerfType{}
	hw_cache_op_result_ids := []PerfType{}

	numEvents := len(configs)

	peConfig := NewPerfEventConfig(numEvents)

	SetupPerfEvents(&peConfig, configs, types, hw_cache_op_ids, hw_cache_op_result_ids, numEvents)

	return peConfig, strings
}

func StartInstrumentation(group_fd int) {
	C.reset_and_enable_ioctl((C.int)(group_fd))
}

func EndInstrumentation(group_fd int, strings []string, peConfig *PerfEventConfig, numEvents int) {

	// Disable and read the event
	values := make([]C.int, numEvents)
	C.disable_ioctl((C.int)(group_fd))

	cStrings := make([]*C.char, len(strings))

	for i, s := range strings {
		cStrings[i] = C.CString(s)                // Convert Go string to C string
		defer C.free(unsafe.Pointer(cStrings[i])) // Free memory when done
	}

	C.get_perf(
		(*C.struct_perf_event_attr)(unsafe.Pointer(&peConfig.pe)),
		&cStrings[0],
		&peConfig.Ids[0],
		(C.int)(numEvents),
		(C.int)(group_fd),
		(*C.int)(unsafe.Pointer(&values[0])),
	)

	for i := 0; i < numEvents; i++ {
		log.Printf("%s: %d\n", strings[i], values[i])
	}
}

func SetupPerfEvents(peConfig *PerfEventConfig,
	configs []PerfType,
	types []PerfType,
	hw_cache_op_ids []PerfType,
	hw_cache_op_result_ids []PerfType,
	numEvents int) {

	cache_id_count := 0
	var cache_hw_event C.uint64_t

	if len(hw_cache_op_ids) != len(hw_cache_op_result_ids) {
		log.Fatal("hw_cache_op_ids must have the same elements as hw_cache_op_result_ids!")
		return
	}
	// hw_cache_op_ids can be empty if PERF_TYPE_HW_CACHE is not specified
	for i := 0; i < numEvents; i++ {
		if types[i] == C.PERF_TYPE_HW_CACHE {
			//uint64_t config_cache_id(uint64_t perf_hw_cache_id, uint64_t perf_hw_cache_op_id, uint64_t perf_hw_cache_op_result_id)
			if cache_id_count > len(hw_cache_op_ids) {
				log.Fatal("not enough hw_cache_op_ids!")
				return
			}
			cache_hw_event = C.config_cache_id((C.uint64_t)(configs[i]),
				(C.uint64_t)(hw_cache_op_ids[cache_id_count]),
				(C.uint64_t)(hw_cache_op_result_ids[cache_id_count]))

			cache_id_count++

			peConfig.Configs[i] = cache_hw_event
		} else {
			peConfig.Configs[i] = (C.uint64_t)(configs[i])
		}
		peConfig.Fds[i] = -1
		peConfig.Types[i] = (C.uint64_t)(types[i])
		peConfig.Ids[i] = C.uint64_t(i)
	}
}

/*
We are concerned about:
1. Executed Instructions (0x8)
2. L1D Accesses (0x4)
3. L1D Hits/Misses
4. L2 Accesses
5. L2 Hits/Misses
*/
func main() {
	// we shall test perf here

	types := []PerfType{PERF_TYPE_HARDWARE,
		PERF_TYPE_HARDWARE,
		PERF_TYPE_HW_CACHE,
		// PERF_TYPE_RAW,
	}

	configs := []PerfType{PERF_COUNT_HW_INSTRUCTIONS,
		PERF_COUNT_HW_CPU_CYCLES,
		PERF_COUNT_HW_CACHE_L1D,
		// 0x17,
	}

	hw_cache_op_ids := []PerfType{PERF_COUNT_HW_CACHE_OP_READ}
	hw_cache_op_result_ids := []PerfType{PERF_COUNT_HW_CACHE_RESULT_ACCESS}

	numEvents := len(configs)

	peConfig := NewPerfEventConfig(numEvents)

	SetupPerfEvents(&peConfig, configs, types, hw_cache_op_ids, hw_cache_op_result_ids, numEvents)

	strings := []string{"Instructions", "Cycles", "L1D Cache Read Accesses", "L2_CACHE_RD"}

	group_fd := ConfigPerf(&peConfig, 0)

	if int(group_fd) < 0 {
		log.Fatal("Error setting group_fd")
		return
	}

	StartInstrumentation(group_fd)

	C.something()

	EndInstrumentation(group_fd, strings, &peConfig, numEvents)

}
