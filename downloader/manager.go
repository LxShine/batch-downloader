package downloader

import (
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"batch-downloader/config"
)

type DownloadManager struct {
	config     *config.Config
	workers    []*DownloadWorker
	taskQueue  chan DownloadTask
	resultChan chan DownloadResult

	progressCallback   func(float64, int, int)
	logCallback        func(string)
	completionCallback func(bool)

	isRunning      atomic.Bool
	isCancelled    atomic.Bool
	totalTasks     int
	completedTasks int32

	wg            sync.WaitGroup
	resultChanMux sync.Mutex // 保护resultChan的关闭
	resultClosed  bool       // 标记resultChan是否已关闭
}

func NewDownloadManager(cfg *config.Config) *DownloadManager {
	return &DownloadManager{
		config:     cfg,
		taskQueue:  make(chan DownloadTask, 1000),
		resultChan: make(chan DownloadResult, 1000),
	}
}

func (dm *DownloadManager) ParseExcel(urlColumn, nameColumns, separator, fileExtension string) ([]DownloadTask, error) {
	parser := NewExcelParser(dm.config)
	return parser.Parse(urlColumn, nameColumns, separator, fileExtension)
}

func (dm *DownloadManager) StartDownload(tasks []DownloadTask) {
	if dm.isRunning.Load() {
		return
	}

	dm.isRunning.Store(true)
	dm.isCancelled.Store(false)
	dm.totalTasks = len(tasks)
	dm.completedTasks = 0
	dm.resultClosed = false // 重置关闭标志

	// 重新创建通道（防止之前被关闭）
	dm.taskQueue = make(chan DownloadTask, 1000)
	dm.resultChan = make(chan DownloadResult, 1000)

	// 启动工作器
	dm.startWorkers()

	// 先启动结果处理
	go dm.processResults()

	// 再发送任务，并在完成后关闭通道
	go func() {
		dm.sendTasks(tasks)
		// 等待所有worker处理完成
		dm.wg.Wait()
		// 只有在非取消状态下才关闭resultChan
		if !dm.isCancelled.Load() {
			dm.closeResultChan()
		}
	}()
}

func (dm *DownloadManager) startWorkers() {
	dm.workers = make([]*DownloadWorker, dm.config.MaxConcurrency)

	for i := 0; i < dm.config.MaxConcurrency; i++ {
		worker := NewDownloadWorker(i+1, dm.config, dm.taskQueue, dm.resultChan)
		dm.workers[i] = worker
		worker.Start(&dm.wg)
	}
}

func (dm *DownloadManager) sendTasks(tasks []DownloadTask) {
	defer close(dm.taskQueue)

	for _, task := range tasks {
		if dm.isCancelled.Load() {
			break
		}

		dm.taskQueue <- task
	}
}

func (dm *DownloadManager) processResults() {
	var successCount, failCount, emptyLinkCount int
	startTime := time.Now()
	lastLogTime := time.Now()

	for {
		select {
		case result, ok := <-dm.resultChan:
			if !ok {
				// 通道已关闭，退出循环
				goto finish
			}

			completed := atomic.AddInt32(&dm.completedTasks, 1)

			// 记录结果
			if result.Success {
				successCount++
			} else {
				failCount++
				// 检查是否是空链接错误
				if strings.Contains(result.Error.Error(), "empty URL") ||
					strings.Contains(result.Error.Error(), "invalid URL") {
					emptyLinkCount++
				}
			}

			// 更新进度（每次都更新）
			progress := float64(completed) / float64(dm.totalTasks)
			dm.updateProgress(progress, int(completed), dm.totalTasks)

			// 限制日志输出频率，减少UI卡顿
			now := time.Now()
			shouldLog := now.Sub(lastLogTime) > 500*time.Millisecond || int(completed) == dm.totalTasks

			if shouldLog {
				if result.Success {
					dm.logCallback(fmt.Sprintf("✓ 成功: %s (%.2f MB)", result.Filename, float64(result.FileSize)/(1024*1024)))
				} else {
					dm.logCallback(fmt.Sprintf("✗ 失败: %s - %v", result.Filename, result.Error))
				}
				lastLogTime = now
			}

			// 每完成0.5秒报告一次统计
			if shouldLog && int(completed)%10 == 0 {
				elapsed := time.Since(startTime)
				rate := float64(completed) / elapsed.Seconds()
				dm.logCallback(fmt.Sprintf("📊 已完成: %d/%d, 成功: %d, 失败: %d, 空链接: %d, 速度: %.1f 个/秒",
					completed, dm.totalTasks, successCount, failCount, emptyLinkCount, rate))
			}
		case <-time.After(100 * time.Millisecond):
			// 每100ms检查一次是否被取消
			if dm.isCancelled.Load() {
				goto finish
			}
		}
	}

finish:
	// 完成处理
	dm.isRunning.Store(false)

	// 报告最终结果（使用绿色加粗字体）
	if dm.isCancelled.Load() {
		// 取消操作
		dm.logCallback(fmt.Sprintf("⛔ 下载已取消! 已完成: %d, **成功: %d, 失败: %d, 空链接: %d**",
			atomic.LoadInt32(&dm.completedTasks), successCount, failCount, emptyLinkCount))
		dm.completionCallback(false)
	} else {
		// 正常完成
		elapsed := time.Since(startTime)
		dm.logCallback(fmt.Sprintf("🎉 **下载完成! 成功: %d, 失败: %d, 空链接: %d, 总耗时: %v**",
			successCount, failCount, emptyLinkCount, elapsed.Round(time.Second)))
		dm.completionCallback(true)
	}
}

func (dm *DownloadManager) Cancel() {
	if !dm.isRunning.Load() {
		return // 如果没有运行，直接返回
	}

	dm.isCancelled.Store(true)
	dm.logCallback("🛑 正在取消下载...")

	// 停止所有工作器
	for _, worker := range dm.workers {
		if worker != nil {
			worker.Stop()
		}
	}

	// 清空任务队列
	go func() {
		for {
			select {
			case <-dm.taskQueue:
				// 消耗任务
			default:
				return
			}
		}
	}()

	// 注意：不关闭resultChan，因为processResults仍在读取
	// processResults会检测到isCancelled并退出
}

// closeResultChan 安全地关闭结果通道
func (dm *DownloadManager) closeResultChan() {
	dm.resultChanMux.Lock()
	defer dm.resultChanMux.Unlock()

	if !dm.resultClosed {
		close(dm.resultChan)
		dm.resultClosed = true
	}
}

func (dm *DownloadManager) SetProgressCallback(callback func(float64, int, int)) {
	dm.progressCallback = callback
}

func (dm *DownloadManager) SetLogCallback(callback func(string)) {
	dm.logCallback = callback
}

func (dm *DownloadManager) SetCompletionCallback(callback func(bool)) {
	dm.completionCallback = callback
}

func (dm *DownloadManager) updateProgress(progress float64, current, total int) {
	if dm.progressCallback != nil {
		dm.progressCallback(progress, current, total)
	}
}

// IsRunning 返回下载是否正在运行
func (dm *DownloadManager) IsRunning() bool {
	return dm.isRunning.Load()
}
