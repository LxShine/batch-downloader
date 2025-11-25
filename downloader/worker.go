package downloader

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"

	"batch-downloader/config"
)

type DownloadWorker struct {
	id         int
	config     *config.Config
	taskQueue  <-chan DownloadTask
	resultChan chan<- DownloadResult
	client     *http.Client
	stopChan   chan struct{}
}

func NewDownloadWorker(id int, cfg *config.Config, taskQueue <-chan DownloadTask, resultChan chan<- DownloadResult) *DownloadWorker {
	client := &http.Client{
		Timeout: time.Duration(cfg.Timeout) * time.Second,
		Transport: &http.Transport{
			MaxIdleConns:        100,
			MaxIdleConnsPerHost: 100,
			IdleConnTimeout:     90 * time.Second,
		},
	}

	return &DownloadWorker{
		id:         id,
		config:     cfg,
		taskQueue:  taskQueue,
		resultChan: resultChan,
		client:     client,
		stopChan:   make(chan struct{}),
	}
}

func (w *DownloadWorker) Start(wg *sync.WaitGroup) {
	wg.Add(1)
	go w.run(wg)
}

func (w *DownloadWorker) run(wg *sync.WaitGroup) {
	defer wg.Done()

	for {
		select {
		case task, ok := <-w.taskQueue:
			if !ok {
				return
			}
			w.processTask(task)
		case <-w.stopChan:
			return
		}
	}
}

func (w *DownloadWorker) processTask(task DownloadTask) {
	result := w.downloadWithRetry(task)
	
	// 安全地发送结果，如果已被取消则跳过
	select {
	case w.resultChan <- result:
		// 成功发送
	case <-w.stopChan:
		// 已被取消，不发送结果
	}
}

func (w *DownloadWorker) downloadWithRetry(task DownloadTask) DownloadResult {
	var lastError error

	for attempt := 1; attempt <= w.config.RetryCount; attempt++ {
		result := w.downloadFile(task)

		if result.Success {
			return result
		}

		lastError = result.Error

		if attempt < w.config.RetryCount {
			// 指数退避
			delay := time.Duration(attempt*attempt) * time.Second
			time.Sleep(delay)
		}
	}

	return DownloadResult{
		Task:     task,
		Success:  false,
		Error:    fmt.Errorf("重试 %d 次后失败: %v", w.config.RetryCount, lastError),
		Filename: task.Filename,
	}
}

func (w *DownloadWorker) downloadFile(task DownloadTask) DownloadResult {
	startTime := time.Now()

	// 确保目录存在
	dir := filepath.Dir(task.SavePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return DownloadResult{
			Task:     task,
			Success:  false,
			Error:    fmt.Errorf("创建目录失败: %v", err),
			Filename: task.Filename,
		}
	}

	// 创建临时文件
	tempFile := task.SavePath + ".tmp"
	file, err := os.Create(tempFile)
	if err != nil {
		return DownloadResult{
			Task:     task,
			Success:  false,
			Error:    fmt.Errorf("创建文件失败: %v", err),
			Filename: task.Filename,
		}
	}

	// 发送请求
	resp, err := w.client.Get(task.URL)
	if err != nil {
		file.Close()
		os.Remove(tempFile)
		return DownloadResult{
			Task:     task,
			Success:  false,
			Error:    fmt.Errorf("请求失败: %v", err),
			Filename: task.Filename,
		}
	}
	defer resp.Body.Close()

	// 检查状态码
	if resp.StatusCode != http.StatusOK {
		file.Close()
		os.Remove(tempFile)
		return DownloadResult{
			Task:     task,
			Success:  false,
			Error:    fmt.Errorf("HTTP %s", resp.Status),
			Filename: task.Filename,
		}
	}

	// 下载文件
	size, err := io.Copy(file, resp.Body)
	if err != nil {
		file.Close()
		os.Remove(tempFile)
		return DownloadResult{
			Task:     task,
			Success:  false,
			Error:    fmt.Errorf("下载失败: %v", err),
			Filename: task.Filename,
		}
	}

	// 关闭文件
	if err := file.Close(); err != nil {
		os.Remove(tempFile)
		return DownloadResult{
			Task:     task,
			Success:  false,
			Error:    fmt.Errorf("关闭文件失败: %v", err),
			Filename: task.Filename,
		}
	}

	// 重命名文件
	if err := os.Rename(tempFile, task.SavePath); err != nil {
		os.Remove(tempFile)
		return DownloadResult{
			Task:     task,
			Success:  false,
			Error:    fmt.Errorf("重命名失败: %v", err),
			Filename: task.Filename,
		}
	}

	duration := time.Since(startTime)

	return DownloadResult{
		Task:     task,
		Success:  true,
		FileSize: size,
		Duration: duration,
		Filename: task.Filename,
	}
}

func (w *DownloadWorker) Stop() {
	close(w.stopChan)
}
