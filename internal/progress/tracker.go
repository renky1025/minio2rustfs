package progress

import (
	"fmt"
	"sync"
	"time"
)

// Status represents the current migration status
type Status struct {
	TotalObjects     int64         // 总对象数量
	ProcessedObjects int64         // 已处理对象数量
	SuccessObjects   int64         // 成功对象数量
	FailedObjects    int64         // 失败对象数量
	SkippedObjects   int64         // 跳过对象数量
	TotalBytes       int64         // 总字节数
	ProcessedBytes   int64         // 已处理字节数
	StartTime        time.Time     // 开始时间
	LastUpdateTime   time.Time     // 最后更新时间
	CurrentSpeed     float64       // 当前速度 (bytes/second)
	AverageSpeed     float64       // 平均速度 (bytes/second)
	ETA              time.Duration // 预计剩余时间
}

// Tracker tracks migration progress
type Tracker struct {
	mu           sync.RWMutex
	status       Status
	speedSamples []speedSample // 用于计算平均速度的样本
	maxSamples   int           // 最大样本数量
}

type speedSample struct {
	timestamp time.Time
	bytes     int64
}

// NewTracker creates a new progress tracker
func NewTracker() *Tracker {
	return &Tracker{
		status: Status{
			StartTime:      time.Now(),
			LastUpdateTime: time.Now(),
		},
		speedSamples: make([]speedSample, 0, 60), // 保存60个样本点
		maxSamples:   60,
	}
}

// SetTotal sets the total number of objects and bytes
func (t *Tracker) SetTotal(objects, bytes int64) {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.status.TotalObjects = objects
	t.status.TotalBytes = bytes
}

// AddSuccess increments successful objects count
func (t *Tracker) AddSuccess(bytes int64) {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.status.SuccessObjects++
	t.status.ProcessedObjects++
	t.status.ProcessedBytes += bytes
	t.updateSpeed(bytes)
}

// AddFailed increments failed objects count
func (t *Tracker) AddFailed() {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.status.FailedObjects++
	t.status.ProcessedObjects++
}

// AddSkipped increments skipped objects count
func (t *Tracker) AddSkipped(bytes int64) {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.status.SkippedObjects++
	t.status.ProcessedObjects++
	t.status.ProcessedBytes += bytes
	t.updateSpeed(bytes)
}

// updateSpeed updates the speed calculation (must be called with lock held)
func (t *Tracker) updateSpeed(bytes int64) {
	now := time.Now()

	// 添加新样本
	t.speedSamples = append(t.speedSamples, speedSample{
		timestamp: now,
		bytes:     bytes,
	})

	// 保持样本数量在限制内
	if len(t.speedSamples) > t.maxSamples {
		t.speedSamples = t.speedSamples[1:]
	}

	// 计算当前速度（基于最近5秒的数据）
	t.calculateCurrentSpeed(now)

	// 计算平均速度
	t.calculateAverageSpeed(now)

	// 计算ETA
	t.calculateETA()

	t.status.LastUpdateTime = now
}

// calculateCurrentSpeed calculates current speed based on recent samples
func (t *Tracker) calculateCurrentSpeed(now time.Time) {
	if len(t.speedSamples) < 2 {
		t.status.CurrentSpeed = 0
		return
	}

	// 计算最近5秒内的速度
	cutoff := now.Add(-5 * time.Second)
	var recentBytes int64
	var recentDuration time.Duration
	var firstSample *speedSample

	for i := len(t.speedSamples) - 1; i >= 0; i-- {
		sample := &t.speedSamples[i]
		if sample.timestamp.Before(cutoff) {
			break
		}
		recentBytes += sample.bytes
		firstSample = sample
	}

	if firstSample != nil {
		recentDuration = now.Sub(firstSample.timestamp)
		if recentDuration > 0 {
			t.status.CurrentSpeed = float64(recentBytes) / recentDuration.Seconds()
		}
	}
}

// calculateAverageSpeed calculates average speed since start
func (t *Tracker) calculateAverageSpeed(now time.Time) {
	elapsed := now.Sub(t.status.StartTime)
	if elapsed > 0 {
		t.status.AverageSpeed = float64(t.status.ProcessedBytes) / elapsed.Seconds()
	}
}

// calculateETA calculates estimated time to completion
func (t *Tracker) calculateETA() {
	if t.status.TotalBytes == 0 || t.status.AverageSpeed == 0 {
		t.status.ETA = 0
		return
	}

	remainingBytes := t.status.TotalBytes - t.status.ProcessedBytes
	if remainingBytes <= 0 {
		t.status.ETA = 0
		return
	}

	etaSeconds := float64(remainingBytes) / t.status.AverageSpeed
	t.status.ETA = time.Duration(etaSeconds) * time.Second
}

// GetStatus returns the current status (thread-safe)
func (t *Tracker) GetStatus() Status {
	t.mu.RLock()
	defer t.mu.RUnlock()

	return t.status
}

// GetProgressPercent returns the progress percentage
func (t *Tracker) GetProgressPercent() float64 {
	t.mu.RLock()
	defer t.mu.RUnlock()

	if t.status.TotalObjects == 0 {
		return 0
	}

	return float64(t.status.ProcessedObjects) / float64(t.status.TotalObjects) * 100
}

// GetBytesProgressPercent returns the bytes progress percentage
func (t *Tracker) GetBytesProgressPercent() float64 {
	t.mu.RLock()
	defer t.mu.RUnlock()

	if t.status.TotalBytes == 0 {
		return 0
	}

	return float64(t.status.ProcessedBytes) / float64(t.status.TotalBytes) * 100
}

// FormatSpeed formats speed in human readable format
func FormatSpeed(bytesPerSecond float64) string {
	if bytesPerSecond < 1024 {
		return fmt.Sprintf("%.1f B/s", bytesPerSecond)
	} else if bytesPerSecond < 1024*1024 {
		return fmt.Sprintf("%.1f KB/s", bytesPerSecond/1024)
	} else if bytesPerSecond < 1024*1024*1024 {
		return fmt.Sprintf("%.1f MB/s", bytesPerSecond/(1024*1024))
	} else {
		return fmt.Sprintf("%.1f GB/s", bytesPerSecond/(1024*1024*1024))
	}
}

// FormatBytes formats bytes in human readable format
func FormatBytes(bytes int64) string {
	if bytes < 1024 {
		return fmt.Sprintf("%d B", bytes)
	} else if bytes < 1024*1024 {
		return fmt.Sprintf("%.1f KB", float64(bytes)/1024)
	} else if bytes < 1024*1024*1024 {
		return fmt.Sprintf("%.1f MB", float64(bytes)/(1024*1024))
	} else {
		return fmt.Sprintf("%.1f GB", float64(bytes)/(1024*1024*1024))
	}
}

// FormatDuration formats duration in human readable format
func FormatDuration(d time.Duration) string {
	if d == 0 {
		return "计算中..."
	}

	hours := int(d.Hours())
	minutes := int(d.Minutes()) % 60
	seconds := int(d.Seconds()) % 60

	if hours > 0 {
		return fmt.Sprintf("%dh%dm%ds", hours, minutes, seconds)
	} else if minutes > 0 {
		return fmt.Sprintf("%dm%ds", minutes, seconds)
	} else {
		return fmt.Sprintf("%ds", seconds)
	}
}
