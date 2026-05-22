package controller

import (
	"bytes"
	"fmt"
	"net/http"
	"runtime"
	"runtime/pprof"
	"runtime/trace"
	"strconv"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

var cpuProfileMu sync.Mutex
var traceMu sync.Mutex

func DownloadCPUProfile(c *gin.Context) {
	seconds := clampIntQuery(c, "seconds", 10, 1, 120)
	if seconds == 0 {
		return
	}

	cpuProfileMu.Lock()
	defer cpuProfileMu.Unlock()

	var buf bytes.Buffer
	if err := pprof.StartCPUProfile(&buf); err != nil {
		c.JSON(http.StatusConflict, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}
	defer pprof.StopCPUProfile()

	timer := time.NewTimer(time.Duration(seconds) * time.Second)
	defer timer.Stop()

	select {
	case <-timer.C:
	case <-c.Request.Context().Done():
	}

	sendProfile(c, fmt.Sprintf("cpu-%s.pprof", time.Now().Format("20060102150405")), buf.Bytes())
}

func DownloadHeapProfile(c *gin.Context) {
	debug := clampIntQuery(c, "debug", 0, 0, 2)
	if debug == 0 && c.IsAborted() {
		return
	}

	if parseBoolQuery(c, "gc", false) {
		runtime.GC()
	}

	var buf bytes.Buffer
	if err := pprof.Lookup("heap").WriteTo(&buf, debug); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}

	sendProfile(c, fmt.Sprintf("heap-%s.pprof", time.Now().Format("20060102150405")), buf.Bytes())
}

func DownloadGoroutineProfile(c *gin.Context) {
	debug := clampIntQuery(c, "debug", 2, 0, 2)
	if debug == 0 && c.IsAborted() {
		return
	}

	var buf bytes.Buffer
	if err := pprof.Lookup("goroutine").WriteTo(&buf, debug); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}

	sendProfile(c, fmt.Sprintf("goroutine-%s.pprof", time.Now().Format("20060102150405")), buf.Bytes())
}

func DownloadMutexProfile(c *gin.Context) {
	debug := clampIntQuery(c, "debug", 0, 0, 2)
	if debug == 0 && c.IsAborted() {
		return
	}

	var buf bytes.Buffer
	if err := pprof.Lookup("mutex").WriteTo(&buf, debug); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}

	sendProfile(c, fmt.Sprintf("mutex-%s.pprof", time.Now().Format("20060102150405")), buf.Bytes())
}

func DownloadBlockProfile(c *gin.Context) {
	debug := clampIntQuery(c, "debug", 0, 0, 2)
	if debug == 0 && c.IsAborted() {
		return
	}

	var buf bytes.Buffer
	if err := pprof.Lookup("block").WriteTo(&buf, debug); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}

	sendProfile(c, fmt.Sprintf("block-%s.pprof", time.Now().Format("20060102150405")), buf.Bytes())
}

func DownloadTrace(c *gin.Context) {
	seconds := clampIntQuery(c, "seconds", 5, 1, 60)
	if seconds == 0 {
		return
	}

	traceMu.Lock()
	defer traceMu.Unlock()

	var buf bytes.Buffer
	if err := trace.Start(&buf); err != nil {
		c.JSON(http.StatusConflict, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}
	defer trace.Stop()

	timer := time.NewTimer(time.Duration(seconds) * time.Second)
	defer timer.Stop()

	select {
	case <-timer.C:
	case <-c.Request.Context().Done():
	}

	sendProfile(c, fmt.Sprintf("trace-%s.out", time.Now().Format("20060102150405")), buf.Bytes())
}

func sendProfile(c *gin.Context, filename string, data []byte) {
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%q", filename))
	c.Data(http.StatusOK, "application/octet-stream", data)
}

func clampIntQuery(c *gin.Context, key string, defaultValue, minValue, maxValue int) int {
	raw := c.Query(key)
	if raw == "" {
		return defaultValue
	}
	parsed, err := strconv.Atoi(raw)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": fmt.Sprintf("%s must be an integer", key)})
		c.Abort()
		return 0
	}
	if parsed < minValue || parsed > maxValue {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": fmt.Sprintf("%s must be between %d and %d", key, minValue, maxValue)})
		c.Abort()
		return 0
	}
	return parsed
}

func parseBoolQuery(c *gin.Context, key string, defaultValue bool) bool {
	raw := c.Query(key)
	if raw == "" {
		return defaultValue
	}
	parsed, err := strconv.ParseBool(raw)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": fmt.Sprintf("%s must be a boolean", key)})
		c.Abort()
		return defaultValue
	}
	return parsed
}
