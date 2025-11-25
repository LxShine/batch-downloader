package gui

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"batch-downloader/config"
	"batch-downloader/downloader"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"
)

// UI 常量
const (
	appID              = "com.batchdownloader.app"
	appTitle           = "批量文件下载器"
	windowWidth        = 1100
	windowHeight       = 750
	logAreaHeight      = 350
	logAreaWidth       = 1050
	minConcurrency     = 1
	maxConcurrency     = 50
	defaultConcurrency = 10
	maxLogLines        = 500 // 最大日志行数
)

// 默认值常量
const (
	defaultURLColumn      = "A"
	defaultNameColumns    = "B,C,D"
	defaultSeparator      = "_"
	defaultConcurrencyStr = "10"
)

type App struct {
	fyneApp    fyne.App
	mainWindow fyne.Window
	config     *config.Config

	// UI 组件
	excelPathEntry     *widget.Entry
	outputDirEntry     *widget.Entry
	urlColumnSelect    *widget.Select     // 修Dollar为下拉框
	nameColumnsCheck   *widget.CheckGroup // 改为多选框
	separatorEntry     *widget.Entry
	concurrencyEntry   *widget.Entry
	fileExtensionEntry *widget.Entry

	progressBar *widget.ProgressBar
	statusLabel *widget.Label
	logText     *widget.Entry

	downloadBtn *widget.Button
	cancelBtn   *widget.Button

	// Excel表头数据
	excelHeaders []string

	downloadManager *downloader.DownloadManager

	// UI更新节流
	lastProgressUpdate time.Time
	progressMutex      sync.Mutex

	// 性能统计
	downloadStartTime  time.Time
	lastCompletedCount int
}

func NewApp() *App {
	fyneApp := app.NewWithID(appID)
	mainWindow := fyneApp.NewWindow(appTitle)
	mainWindow.Resize(fyne.NewSize(windowWidth, windowHeight))

	// 设置窗口图标
	if icon := loadAppIcon(); icon != nil {
		mainWindow.SetIcon(icon)
	}

	cfg := config.NewConfig()

	return &App{
		fyneApp:    fyneApp,
		mainWindow: mainWindow,
		config:     cfg,
	}
}

func (a *App) Run() error {
	a.setupUI()
	a.mainWindow.ShowAndRun()
	return nil
}

func (a *App) setupUI() {
	// 创建 UI 组件
	a.createComponents()

	// 布局
	form := a.createForm()
	progressArea := a.createProgressArea()
	logArea := a.createLogArea()

	content := container.NewBorder(
		form,
		progressArea,
		nil,
		nil,
		logArea,
	)

	a.mainWindow.SetContent(content)
}

func (a *App) createComponents() {
	// Excel 文件选择
	a.excelPathEntry = a.createEntry(a.config.ExcelPath, "", false)
	a.excelPathEntry.OnChanged = func(s string) {
		a.loadExcelHeaders()
	}

	// 输出目录选择
	a.outputDirEntry = a.createEntry(a.config.OutputDir, "", true)

	// URL列下拉选择框
	a.urlColumnSelect = widget.NewSelect([]string{}, func(value string) {
		a.validateInputs("")
	})
	a.urlColumnSelect.PlaceHolder = "请先选择Excel文件"

	// 文件名列多选框
	a.nameColumnsCheck = widget.NewCheckGroup([]string{}, func(selected []string) {
		a.validateInputs("")
	})
	a.nameColumnsCheck.Horizontal = true // 水平显示

	// 其他配置
	a.separatorEntry = a.createEntry(defaultSeparator, "列分隔符", false)
	a.fileExtensionEntry = a.createEntry("", "文件扩展名 (如: pdf, jpg, 留空则从URL推断)", false)

	// 并发配置
	a.concurrencyEntry = a.createEntry(fmt.Sprintf("%d", a.config.MaxConcurrency), "", false)

	// 进度组件
	a.progressBar = widget.NewProgressBar()
	a.progressBar.Min = 0
	a.progressBar.Max = 1
	a.progressBar.TextFormatter = func() string {
		pct := a.progressBar.Value * 100
		if pct >= 100 {
			return "✅ 100%"
		}
		if pct > 0 {
			return fmt.Sprintf("🔄 %.1f%%", pct)
		}
		return "⏳ 0%"
	}
	a.statusLabel = widget.NewLabelWithStyle(
		"⚙️ 准备就绪",
		fyne.TextAlignLeading,
		fyne.TextStyle{},
	)

	// 日志区域
	a.logText = widget.NewMultiLineEntry()
	a.logText.SetPlaceHolder("📄 下载日志将在这里显示...")
	a.logText.Disable()
	a.logText.Wrapping = fyne.TextWrapWord

	// 按钮
	a.downloadBtn = widget.NewButton("🚀 开始下载", a.startDownload)
	a.downloadBtn.Importance = widget.HighImportance
	a.downloadBtn.Disable()

	a.cancelBtn = widget.NewButton("❌ 取消下载", a.cancelDownload)
	a.cancelBtn.Importance = widget.DangerImportance
	a.cancelBtn.Disable()
}

func (a *App) createForm() *widget.Form {
	// 创建文件名列的美化容器
	nameColumnsHint := widget.NewLabel("勾选用于组成文件名的列，按选中顺序拼接")
	nameColumnsHint.TextStyle = fyne.TextStyle{Italic: true}

	nameColumnsCard := container.NewVBox(
		a.nameColumnsCheck,
		widget.NewSeparator(),
		nameColumnsHint,
	)
	nameColumnsScroll := container.NewScroll(nameColumnsCard)
	nameColumnsScroll.SetMinSize(fyne.NewSize(450, 120))

	return widget.NewForm(
		widget.NewFormItem("📂 Excel 文件", container.NewBorder(nil, nil, nil,
			widget.NewButton("📁 浏览", a.browseExcelFile),
			a.excelPathEntry)),
		widget.NewFormItem("📁 输出目录", container.NewBorder(nil, nil, nil,
			widget.NewButton("📁 浏览", a.browseOutputDir),
			a.outputDirEntry)),
		widget.NewFormItem("🔗 下载链接列", a.urlColumnSelect),
		widget.NewFormItem("📝 文件名组成列", nameColumnsScroll),
		widget.NewFormItem("➕ 文件名分隔符", a.separatorEntry),
		widget.NewFormItem("📎 文件扩展名", a.fileExtensionEntry),
		widget.NewFormItem("⚡ 并发下载数", a.concurrencyEntry),
	)
}

func (a *App) createProgressArea() *fyne.Container {
	// 创建进度条容器，增加内边距
	progressContainer := container.NewPadded(
		container.NewVBox(
			a.progressBar,
		),
	)

	// 状态和按钮区域
	controlArea := container.NewBorder(
		nil, nil,
		a.statusLabel,
		container.NewHBox(
			a.downloadBtn,
			a.cancelBtn,
		),
		nil,
	)

	return container.NewVBox(
		progressContainer,
		controlArea,
	)
}

func (a *App) createLogArea() *container.Scroll {
	// 创建日志头部
	logHeader := widget.NewLabelWithStyle(
		"📊 下载日志",
		fyne.TextAlignLeading,
		fyne.TextStyle{Bold: true},
	)

	// 日志内容区域
	logContainer := container.NewBorder(
		container.NewVBox(logHeader, widget.NewSeparator()),
		nil, nil, nil,
		a.logText,
	)

	scroll := container.NewScroll(logContainer)
	scroll.SetMinSize(fyne.NewSize(logAreaWidth, logAreaHeight))
	return scroll
}

// createEntry 创建输入框的辅助方法
func (a *App) createEntry(text, placeholder string, validate bool) *widget.Entry {
	entry := widget.NewEntry()
	if text != "" {
		entry.SetText(text)
	}
	if placeholder != "" {
		entry.SetPlaceHolder(placeholder)
	}
	if validate {
		entry.OnChanged = a.validateInputs
	}
	return entry
}

func (a *App) browseExcelFile() {
	dialog.ShowFileOpen(func(reader fyne.URIReadCloser, err error) {
		if err != nil || reader == nil {
			return
		}
		a.excelPathEntry.SetText(reader.URI().Path())
		reader.Close()
		// 自动加载表头
		a.loadExcelHeaders()
	}, a.mainWindow)
}

func (a *App) browseOutputDir() {
	dialog.ShowFolderOpen(func(list fyne.ListableURI, err error) {
		if err != nil || list == nil {
			return
		}
		a.outputDirEntry.SetText(list.Path())
	}, a.mainWindow)
}

func (a *App) validateInputs(_ string) {
	hasExcel := strings.TrimSpace(a.excelPathEntry.Text) != ""
	hasOutput := strings.TrimSpace(a.outputDirEntry.Text) != ""
	hasURLCol := a.urlColumnSelect.Selected != ""
	hasNameCols := len(a.nameColumnsCheck.Selected) > 0

	canDownload := hasExcel && hasOutput && hasURLCol && hasNameCols

	if canDownload {
		a.downloadBtn.Enable()
	} else {
		a.downloadBtn.Disable()
	}
}

// loadExcelHeaders 加载Excel表头
func (a *App) loadExcelHeaders() {
	excelPath := strings.TrimSpace(a.excelPathEntry.Text)
	if excelPath == "" || !a.isValidPath(excelPath) {
		return
	}

	// 读取表头
	headers, err := downloader.ReadExcelHeaders(excelPath)
	if err != nil {
		a.showError("读取失败", fmt.Sprintf("无法读取Excel表头: %v", err))
		return
	}

	if len(headers) == 0 {
		a.showError("读取失败", "Excel文件为空或没有表头")
		return
	}

	a.excelHeaders = headers

	// 更新URL列下拉框
	a.urlColumnSelect.Options = headers
	if len(headers) > 0 {
		a.urlColumnSelect.SetSelected(headers[0]) // 默认选择第一列
	}
	a.urlColumnSelect.Refresh()

	// 更新文件名列多选框
	a.nameColumnsCheck.Options = headers
	if len(headers) > 1 {
		// 默认选中第2-4列
		defaultSelected := []string{}
		for i := 1; i < len(headers) && i < 4; i++ {
			defaultSelected = append(defaultSelected, headers[i])
		}
		a.nameColumnsCheck.Selected = defaultSelected
	}
	a.nameColumnsCheck.Refresh()

	a.validateInputs("")
	a.addLog(fmt.Sprintf("✓ 已加载Excel表头，共 %d 列", len(headers)))
}

// isValidPath 验证路径是否有效
func (a *App) isValidPath(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// showError 显示错误对话框
func (a *App) showError(title, message string) {
	dialog.ShowError(fmt.Errorf("%s", message), a.mainWindow)
}

// showInfo 显示信息对话框
func (a *App) showInfo(title, message string) {
	dialog.ShowInformation(title, message, a.mainWindow)
}

func (a *App) startDownload() {
	// 验证输入
	if err := a.validateBeforeDownload(); err != nil {
		a.showError("验证失败", err.Error())
		return
	}

	// 更新配置
	a.config.ExcelPath = strings.TrimSpace(a.excelPathEntry.Text)
	a.config.OutputDir = strings.TrimSpace(a.outputDirEntry.Text)
	a.config.MaxConcurrency = a.getConcurrency()

	// 更新 UI 状态
	a.setDownloadingState(true)

	// 清空日志和进度
	a.resetProgress()

	// 创建下载管理器
	a.downloadManager = downloader.NewDownloadManager(a.config)

	// 设置回调
	a.setupCallbacks()

	// 开始下载
	go a.executeDownload()
}

// validateBeforeDownload 下载前验证
func (a *App) validateBeforeDownload() error {
	excelPath := strings.TrimSpace(a.excelPathEntry.Text)
	outputDir := strings.TrimSpace(a.outputDirEntry.Text)

	// 验证 Excel 文件
	if !a.isValidPath(excelPath) {
		return fmt.Errorf("Excel 文件不存在: %s", excelPath)
	}

	// 验证输出目录
	if outputDir != "" {
		if err := os.MkdirAll(outputDir, 0755); err != nil {
			return fmt.Errorf("无法创建输出目录: %v", err)
		}
	}

	// 验证 URL 列
	urlCol := a.urlColumnSelect.Selected
	if urlCol == "" {
		return fmt.Errorf("URL 列不能为空")
	}

	// 验证文件名列
	if len(a.nameColumnsCheck.Selected) == 0 {
		return fmt.Errorf("文件名列不能为空")
	}

	return nil
}

// setDownloadingState 设置下载状态
func (a *App) setDownloadingState(isDownloading bool) {
	if isDownloading {
		a.downloadBtn.Disable()
		a.cancelBtn.Enable()
	} else {
		a.downloadBtn.Enable()
		a.cancelBtn.Disable()
	}
}

// resetProgress 重置进度显示
func (a *App) resetProgress() {
	a.logText.SetText("")
	a.progressBar.SetValue(0)
	a.statusLabel.SetText("🚀 准备开始下载...")
	a.downloadStartTime = time.Now()
	a.lastCompletedCount = 0
}

// setupCallbacks 设置回调函数
func (a *App) setupCallbacks() {
	a.downloadManager.SetProgressCallback(a.updateProgress)
	a.downloadManager.SetLogCallback(a.addLog)
	a.downloadManager.SetCompletionCallback(a.downloadComplete)
}

// executeDownload 执行下载任务
func (a *App) executeDownload() {
	// 获取选中的列
	urlColumn := a.urlColumnSelect.Selected
	nameColumns := strings.Join(a.nameColumnsCheck.Selected, ",")

	tasks, err := a.downloadManager.ParseExcel(
		urlColumn,
		nameColumns,
		strings.TrimSpace(a.separatorEntry.Text),
		strings.TrimSpace(a.fileExtensionEntry.Text),
	)
	if err != nil {
		a.addLog(fmt.Sprintf("❌ 解析Excel失败: %v", err))
		a.downloadComplete(false)
		return
	}

	if len(tasks) == 0 {
		a.addLog("⚠️  未找到有效的下载任务")
		a.downloadComplete(false)
		return
	}

	a.addLog(fmt.Sprintf("📋 找到 %d 个下载任务，开始下载...", len(tasks)))
	a.downloadManager.StartDownload(tasks)
}

func (a *App) cancelDownload() {
	if a.downloadManager != nil && a.downloadManager.IsRunning() {
		a.addLog("🛑 正在取消下载...")
		// Cancel()会阻塞直到所有worker停止，然后会触发completionCallback
		// 所以这里不需要调用downloadComplete
		a.downloadManager.Cancel()
	}
}

func (a *App) downloadComplete(success bool) {
	a.setDownloadingState(false)

	if success {
		a.statusLabel.SetText("✅ 下载完成")
		a.progressBar.SetValue(1.0)
	} else {
		a.statusLabel.SetText("⛔ 下载已停止")
	}
}

func (a *App) updateProgress(progress float64, current, total int) {
	a.progressMutex.Lock()
	defer a.progressMutex.Unlock()

	// 节流: 每200ms最多更新一次，除非是完成状态
	now := time.Now()
	isComplete := current == total
	if !isComplete && now.Sub(a.lastProgressUpdate) < 200*time.Millisecond {
		return // 跳过过于频繁的更新
	}
	a.lastProgressUpdate = now

	// 限制进度范围
	if progress < 0 {
		progress = 0
	} else if progress > 1 {
		progress = 1
	}

	// 更新进度条
	a.progressBar.SetValue(progress)

	// 计算统计信息
	statusText := a.buildStatusText(progress, current, total, now)
	a.statusLabel.SetText(statusText)
}

// buildStatusText 构建状态文本（包含进度、速度、预估时间）
func (a *App) buildStatusText(progress float64, current, total int, now time.Time) string {
	if current == 0 {
		return "🚀 正在启动..."
	}

	// 基本进度信息
	progressPct := progress * 100
	baseText := fmt.Sprintf("📊 进度: %d/%d (%.1f%%)", current, total, progressPct)

	// 计算下载速度
	elapsed := now.Sub(a.downloadStartTime).Seconds()
	if elapsed > 0 {
		speed := float64(current) / elapsed

		// 预估剩余时间
		if current < total && speed > 0 {
			remaining := total - current
			etaSeconds := float64(remaining) / speed
			eta := a.formatDuration(time.Duration(etaSeconds * float64(time.Second)))

			return fmt.Sprintf("%s | ⚡ %.1f 个/秒 | ⏱️ 预计剩余: %s", baseText, speed, eta)
		}

		// 完成时只显示平均速度
		if current == total {
			return fmt.Sprintf("%s | ⚡ 平均 %.1f 个/秒", baseText, speed)
		}

		return fmt.Sprintf("%s | ⚡ %.1f 个/秒", baseText, speed)
	}

	return baseText
}

// formatDuration 格式化时间段
func (a *App) formatDuration(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%.0f秒", d.Seconds())
	}
	if d < time.Hour {
		minutes := int(d.Minutes())
		seconds := int(d.Seconds()) % 60
		return fmt.Sprintf("%d分%d秒", minutes, seconds)
	}
	hours := int(d.Hours())
	minutes := int(d.Minutes()) % 60
	return fmt.Sprintf("%d小时%d分", hours, minutes)
}

func (a *App) addLog(message string) {
	currentText := a.logText.Text
	lines := []string{}

	if currentText != "" {
		lines = strings.Split(currentText, "\n")
	}

	// 添加新日志
	lines = append(lines, message)

	// 限制日志行数，防止卡顿
	if len(lines) > maxLogLines {
		lines = lines[len(lines)-maxLogLines:]
	}

	newText := strings.Join(lines, "\n")
	a.logText.SetText(newText)

	// 滚动到底部
	a.logText.CursorRow = len(lines) - 1
}

func (a *App) getConcurrency() int {
	text := strings.TrimSpace(a.concurrencyEntry.Text)
	concurrency, err := strconv.Atoi(text)
	if err != nil || concurrency < minConcurrency {
		return defaultConcurrency
	}
	if concurrency > maxConcurrency {
		return maxConcurrency
	}
	return concurrency
}

// loadAppIcon 加载应用图标
func loadAppIcon() fyne.Resource {
	// 尝试加载 icon.png
	if iconData := tryLoadIconFile("icon.png"); iconData != nil {
		return fyne.NewStaticResource("icon.png", iconData)
	}

	// 尝试加载 icon.ico （虽然Fyne不直接支持ico，但会尝试）
	if iconData := tryLoadIconFile("icon.ico"); iconData != nil {
		return fyne.NewStaticResource("icon.ico", iconData)
	}

	// 如果没有找到图标文件，返回nil（使用默认图标）
	return nil
}

// tryLoadIconFile 尝试加载图标文件
func tryLoadIconFile(filename string) []byte {
	// 获取可执行文件所在目录
	exePath, err := os.Executable()
	if err != nil {
		return nil
	}
	exeDir := filepath.Dir(exePath)

	// 尝试加载图标
	iconPath := filepath.Join(exeDir, filename)
	data, err := os.ReadFile(iconPath)
	if err == nil {
		return data
	}

	// 如果在exe目录找不到，尝试当前工作目录
	data, err = os.ReadFile(filename)
	if err == nil {
		return data
	}

	return nil
}
