// Command bench is the `make bench` entry point. At H1 it prints the benchmark
// methodology header and machine specs, honestly stating that the measured loop
// is not implemented yet. The real firehose + 60s sustained run lands at H4-H6.
package main

import (
	"fmt"
	"runtime"
)

func main() {
	fmt.Println("=== Palk Watch benchmark ===")
	fmt.Printf("Go:   %s\n", runtime.Version())
	fmt.Printf("OS:   %s/%s\n", runtime.GOOS, runtime.GOARCH)
	fmt.Printf("CPUs: %d\n", runtime.NumCPU())
	fmt.Println()
	fmt.Println("Methodology (PRD section 7, benchmark commitment):")
	fmt.Println("  - Generator in-process, pre-generated messages in memory, no network in the measured loop.")
	fmt.Println("  - 60-second sustained run.")
	fmt.Println("  - Report msgs/sec, inline p50/p99 and sweep p50/p99 (microseconds).")
	fmt.Println("  - Report ingested / processed / dropped counts. All of them, always.")
	fmt.Println("  - If under 50k, print the real number. Never inflate, never cherry-pick a burst peak.")
	fmt.Println()
	fmt.Println("STATUS: harness stub. Real firehose + measured loop lands at H4-H6.")
}
