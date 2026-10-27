// +build linux

package collector

import (
	"bufio"
	"os"
	"strconv"
	"strings"
	"syscall"
)

func getDiskUsage(path string) (total, used uint64, percent float64) {
	var stat syscall.Statfs_t
	if err := syscall.Statfs(path, &stat); err != nil {
		return 0, 0, 0
	}
	total = stat.Blocks * uint64(stat.Bsize)
	free := stat.Bavail * uint64(stat.Bsize)
	used = total - free
	if total > 0 {
		percent = float64(used) / float64(total) * 100
	}
	return
}

func getSystemMemory() (total, used uint64) {
	f, err := os.Open("/proc/meminfo")
	if err != nil {
		return 0, 0
	}
	defer f.Close()

	var memTotal, memAvail uint64
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "MemTotal:") {
			memTotal = parseMemLine(line)
		} else if strings.HasPrefix(line, "MemAvailable:") {
			memAvail = parseMemLine(line)
		}
	}
	total = memTotal * 1024
	used = (memTotal - memAvail) * 1024
	return
}

func parseMemLine(line string) uint64 {
	fields := strings.Fields(line)
	if len(fields) < 2 {
		return 0
	}
	v, _ := strconv.ParseUint(fields[1], 10, 64)
	return v
}
