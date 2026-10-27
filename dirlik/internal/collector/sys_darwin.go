// +build darwin

package collector

import (
	"encoding/binary"
	"runtime"
	"syscall"
	"unsafe"
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
	// macOS: sysctl ile toplam RAM
	val, err := syscall.Sysctl("hw.memsize")
	if err == nil && len(val) >= 8 {
		total = binary.LittleEndian.Uint64([]byte(val[:8]))
	} else if err == nil && len(val) >= int(unsafe.Sizeof(uint64(0))) {
		// Bazı macOS sürümleri farklı uzunluk döndürebilir
		buf := make([]byte, 8)
		copy(buf, []byte(val))
		total = binary.LittleEndian.Uint64(buf)
	}
	// Kullanılan bellek: runtime.MemStats ile yaklaşık
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	used = m.Sys
	return
}
