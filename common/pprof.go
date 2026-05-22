package common

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"runtime/pprof"
	"time"

	"github.com/shirou/gopsutil/cpu"
	"github.com/shirou/gopsutil/mem"
)

// Monitor 定时监控cpu使用率，超过阈值输出pprof文件
func Monitor() {
	outputDir := GetEnvOrDefaultString("PPROF_OUTPUT_DIR", "./pprof")
	intervalSeconds := GetEnvOrDefault("PPROF_MONITOR_INTERVAL_SECONDS", 30)
	if intervalSeconds < 5 {
		intervalSeconds = 5
	}
	cooldownSeconds := GetEnvOrDefault("PPROF_MONITOR_COOLDOWN_SECONDS", 300)
	if cooldownSeconds < 0 {
		cooldownSeconds = 0
	}
	cpuProfileSeconds := GetEnvOrDefault("PPROF_CPU_DURATION_SECONDS", 10)
	if cpuProfileSeconds < 1 {
		cpuProfileSeconds = 1
	}
	if cpuProfileSeconds > 120 {
		cpuProfileSeconds = 120
	}

	var lastCPU time.Time
	var lastHeap time.Time

	for {
		config := GetPerformanceMonitorConfig()
		if !config.Enabled {
			time.Sleep(30 * time.Second)
			continue
		}

		now := time.Now()
		cooldown := time.Duration(cooldownSeconds) * time.Second

		if outputDir != "" {
			if err := os.MkdirAll(outputDir, os.ModePerm); err != nil {
				SysLog("创建pprof文件夹失败 " + err.Error())
			}
		}

		// CPU
		if config.CPUThreshold > 0 && (cooldown == 0 || lastCPU.IsZero() || now.Sub(lastCPU) >= cooldown) {
			percent, err := cpu.Percent(time.Second, false)
			if err != nil {
				SysLog("获取cpu使用率失败 " + err.Error())
			} else if len(percent) > 0 && int(percent[0]) > config.CPUThreshold {
				cpuPath := filepath.Join(outputDir, fmt.Sprintf("cpu-%s.pprof", time.Now().Format("20060102150405")))
				SysLog(fmt.Sprintf("cpu usage too high (current: %.1f%%, threshold: %d%%), writing cpu profile: %s", percent[0], config.CPUThreshold, cpuPath))
				if err := writeCPUProfile(cpuPath, time.Duration(cpuProfileSeconds)*time.Second); err != nil {
					SysLog("写入cpu pprof失败 " + err.Error())
				} else {
					lastCPU = time.Now()
				}
			}
		}

		// Memory (heap)
		if config.MemoryThreshold > 0 && (cooldown == 0 || lastHeap.IsZero() || now.Sub(lastHeap) >= cooldown) {
			vm, err := mem.VirtualMemory()
			if err != nil {
				SysLog("获取内存使用率失败 " + err.Error())
			} else if int(vm.UsedPercent) > config.MemoryThreshold {
				heapPath := filepath.Join(outputDir, fmt.Sprintf("heap-%s.pprof", time.Now().Format("20060102150405")))
				SysLog(fmt.Sprintf("memory usage too high (current: %.1f%%, threshold: %d%%), writing heap profile: %s", vm.UsedPercent, config.MemoryThreshold, heapPath))
				if err := writeHeapProfile(heapPath); err != nil {
					SysLog("写入heap pprof失败 " + err.Error())
				} else {
					lastHeap = time.Now()
				}
			}
		}

		time.Sleep(time.Duration(intervalSeconds) * time.Second)
	}
}

func writeCPUProfile(path string, duration time.Duration) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	if err := pprof.StartCPUProfile(f); err != nil {
		return err
	}
	time.Sleep(duration)
	pprof.StopCPUProfile()
	return nil
}

func writeHeapProfile(path string) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	if GetEnvOrDefaultBool("PPROF_HEAP_GC_BEFORE", false) {
		runtime.GC()
	}

	return pprof.Lookup("heap").WriteTo(f, 0)
}
