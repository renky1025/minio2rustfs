package progress

import (
	"fmt"
	"os"
	"strings"
	"time"
)

// Display handles the progress display
type Display struct {
	tracker   *Tracker
	interval  time.Duration
	stopCh    chan struct{}
	lastLines int // 记录上次输出的行数，用于清屏
}

// NewDisplay creates a new progress display
func NewDisplay(tracker *Tracker, interval time.Duration) *Display {
	return &Display{
		tracker:  tracker,
		interval: interval,
		stopCh:   make(chan struct{}),
	}
}

// Start starts the progress display
func (d *Display) Start() {
	go d.displayLoop()
}

// Stop stops the progress display
func (d *Display) Stop() {
	close(d.stopCh)
}

// displayLoop runs the display update loop
func (d *Display) displayLoop() {
	ticker := time.NewTicker(d.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			d.updateDisplay()
		case <-d.stopCh:
			d.finalDisplay()
			return
		}
	}
}

// updateDisplay updates the console display
func (d *Display) updateDisplay() {
	status := d.tracker.GetStatus()

	// 生成新的显示内容（Windows 下使用简化版本）
	lines := d.generateDisplay(status)

	// 清除上次的输出
	d.clearLines()

	// 输出新内容
	fmt.Print(strings.Join(lines, "\n"))
	d.lastLines = len(lines)
}

// finalDisplay shows the final progress
func (d *Display) finalDisplay() {
	d.clearLines()
	status := d.tracker.GetStatus()
	lines := d.generateFinalDisplay(status)
	fmt.Println(strings.Join(lines, "\n"))
}

// clearLines clears the previous output lines
func (d *Display) clearLines() {
	// Windows CMD 对 ANSI 转义序列支持有限，使用简化方法
	// 直接输出一些换行，让新内容覆盖旧内容
	if d.lastLines > 0 {
		// 在 Windows 下，简单地添加一些空行来"清理"屏幕
		fmt.Print("\n")
	}
}

// generateDisplay generates the progress display lines
func (d *Display) generateDisplay(status Status) []string {
	lines := make([]string, 0)

	// 标题
	lines = append(lines, "")
	lines = append(lines, "🚀 对象迁移进度")
	lines = append(lines, "="+strings.Repeat("=", 50))

	// 对象统计
	objectProgress := d.tracker.GetProgressPercent()
	lines = append(lines, fmt.Sprintf("📊 对象进度: %d/%d (%.1f%%)",
		status.ProcessedObjects, status.TotalObjects, objectProgress))

	// 进度条
	progressBar := d.generateProgressBar(objectProgress, 40)
	lines = append(lines, fmt.Sprintf("    %s", progressBar))

	// 字节统计
	bytesProgress := d.tracker.GetBytesProgressPercent()
	lines = append(lines, fmt.Sprintf("💾 数据进度: %s/%s (%.1f%%)",
		FormatBytes(status.ProcessedBytes), FormatBytes(status.TotalBytes), bytesProgress))

	// 字节进度条
	bytesProgressBar := d.generateProgressBar(bytesProgress, 40)
	lines = append(lines, fmt.Sprintf("    %s", bytesProgressBar))

	// 详细统计
	lines = append(lines, "")
	lines = append(lines, "📈 详细统计:")
	lines = append(lines, fmt.Sprintf("  ✅ 成功: %d", status.SuccessObjects))
	lines = append(lines, fmt.Sprintf("  ❌ 失败: %d", status.FailedObjects))
	lines = append(lines, fmt.Sprintf("  ⏭️  跳过: %d", status.SkippedObjects))

	// 速度信息
	lines = append(lines, "")
	lines = append(lines, "⚡ 速度信息:")
	lines = append(lines, fmt.Sprintf("  当前速度: %s", FormatSpeed(status.CurrentSpeed)))
	lines = append(lines, fmt.Sprintf("  平均速度: %s", FormatSpeed(status.AverageSpeed)))

	// 时间信息
	elapsed := time.Since(status.StartTime)
	lines = append(lines, "")
	lines = append(lines, "⏱️  时间信息:")
	lines = append(lines, fmt.Sprintf("  已用时间: %s", FormatDuration(elapsed)))
	lines = append(lines, fmt.Sprintf("  预计剩余: %s", FormatDuration(status.ETA)))

	// 如果有预计完成时间
	if status.ETA > 0 {
		estimatedCompletion := time.Now().Add(status.ETA)
		lines = append(lines, fmt.Sprintf("  预计完成: %s", estimatedCompletion.Format("15:04:05")))
	}

	lines = append(lines, "")
	lines = append(lines, fmt.Sprintf("⏰ 最后更新: %s", status.LastUpdateTime.Format("15:04:05")))
	lines = append(lines, "")

	return lines
}

// generateFinalDisplay generates the final completion display
func (d *Display) generateFinalDisplay(status Status) []string {
	lines := make([]string, 0)

	elapsed := time.Since(status.StartTime)

	lines = append(lines, "")
	lines = append(lines, "🎉 迁移完成!")
	lines = append(lines, "="+strings.Repeat("=", 50))

	lines = append(lines, fmt.Sprintf("📊 总计处理: %d 个对象", status.ProcessedObjects))
	lines = append(lines, fmt.Sprintf("💾 总计数据: %s", FormatBytes(status.ProcessedBytes)))
	lines = append(lines, fmt.Sprintf("✅ 成功: %d", status.SuccessObjects))
	lines = append(lines, fmt.Sprintf("❌ 失败: %d", status.FailedObjects))
	lines = append(lines, fmt.Sprintf("⏭️  跳过: %d", status.SkippedObjects))
	lines = append(lines, fmt.Sprintf("⏱️  总用时: %s", FormatDuration(elapsed)))
	lines = append(lines, fmt.Sprintf("⚡ 平均速度: %s", FormatSpeed(status.AverageSpeed)))
	lines = append(lines, "")

	return lines
}

// generateProgressBar generates a visual progress bar
func (d *Display) generateProgressBar(percent float64, width int) string {
	if percent > 100 {
		percent = 100
	}
	if percent < 0 {
		percent = 0
	}

	filled := int(percent * float64(width) / 100)
	bar := strings.Repeat("█", filled) + strings.Repeat("░", width-filled)

	return fmt.Sprintf("[%s] %.1f%%", bar, percent)
}

// IsTerminalSupported checks if the terminal supports progress display
func IsTerminalSupported() bool {
	// 在 Windows 上，简化检查逻辑
	// 检查标准输出是否为终端
	if fileInfo, _ := os.Stdout.Stat(); (fileInfo.Mode() & os.ModeCharDevice) == 0 {
		return false
	}

	// Windows 下总是返回 true，让进度显示工作
	return true
}
