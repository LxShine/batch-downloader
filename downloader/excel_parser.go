package downloader

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"
	"unicode"

	"batch-downloader/config"

	"github.com/xuri/excelize/v2"
)

// ReadExcelHeaders 读取Excel文件的表头
func ReadExcelHeaders(excelPath string) ([]string, error) {
	f, err := excelize.OpenFile(excelPath)
	if err != nil {
		return nil, fmt.Errorf("打开Excel文件失败: %v", err)
	}
	defer f.Close()

	// 获取第一个工作表
	sheetName := f.GetSheetName(0)
	if sheetName == "" {
		return nil, fmt.Errorf("Excel文件中没有工作表")
	}

	// 获取所有行
	rows, err := f.GetRows(sheetName)
	if err != nil {
		return nil, fmt.Errorf("读取Excel数据失败: %v", err)
	}

	if len(rows) == 0 {
		return nil, fmt.Errorf("Excel文件为空")
	}

	// 返回第一行作为表头
	headers := rows[0]
	if len(headers) == 0 {
		return nil, fmt.Errorf("表头为空")
	}

	return headers, nil
}

// ReadExcelSampleData 读取Excel文件的样本数据（用于自动识别）
func ReadExcelSampleData(excelPath string, maxRows int) ([][]string, error) {
	f, err := excelize.OpenFile(excelPath)
	if err != nil {
		return nil, fmt.Errorf("打开Excel文件失败: %v", err)
	}
	defer f.Close()

	// 获取第一个工作表
	sheetName := f.GetSheetName(0)
	if sheetName == "" {
		return nil, fmt.Errorf("Excel文件中没有工作表")
	}

	// 获取所有行
	rows, err := f.GetRows(sheetName)
	if err != nil {
		return nil, fmt.Errorf("读取Excel数据失败: %v", err)
	}

	if len(rows) < 2 {
		return nil, fmt.Errorf("Excel文件没有数据行")
	}

	// 跳过表头，返回样本数据行
	sampleRows := [][]string{}
	endRow := len(rows)
	if maxRows > 0 && maxRows+1 < len(rows) {
		endRow = maxRows + 1 // +1 因为跳过表头
	}

	for i := 1; i < endRow; i++ {
		sampleRows = append(sampleRows, rows[i])
	}

	return sampleRows, nil
}

type ExcelParser struct {
	config *config.Config
}

func NewExcelParser(cfg *config.Config) *ExcelParser {
	return &ExcelParser{
		config: cfg,
	}
}

func (p *ExcelParser) Parse(urlColumn, nameColumns, separator, fileExtension string) ([]DownloadTask, error) {
	f, err := excelize.OpenFile(p.config.ExcelPath)
	if err != nil {
		return nil, fmt.Errorf("打开Excel文件失败: %v", err)
	}
	defer f.Close()

	// 获取第一个工作表
	sheetName := f.GetSheetName(0)
	if sheetName == "" {
		return nil, fmt.Errorf("Excel文件中没有工作表")
	}

	// 获取所有行
	rows, err := f.GetRows(sheetName)
	if err != nil {
		return nil, fmt.Errorf("读取Excel数据失败: %v", err)
	}

	if len(rows) < 2 {
		return nil, fmt.Errorf("Excel文件没有数据行")
	}

	// 读取表头（第一行）
	headers := rows[0]

	// 转换列标识为索引（支持表头名称或列号）
	urlColIndex := p.findColumnIndex(urlColumn, headers)
	if urlColIndex < 0 {
		return nil, fmt.Errorf("无效的URL列: %s", urlColumn)
	}

	nameColIndices := p.findColumnsIndices(nameColumns, headers)
	if len(nameColIndices) == 0 {
		return nil, fmt.Errorf("无效的文件名列: %s", nameColumns)
	}

	var tasks []DownloadTask

	// 从第2行开始（跳过标题行）
	for i, row := range rows[1:] {
		if len(row) <= urlColIndex {
			continue
		}

		url := strings.TrimSpace(row[urlColIndex])
		if url == "" {
			continue
		}

		// 构建文件名
		filename := p.buildFilename(row, nameColIndices, separator)
		// 不再跳过文件名为空的行，因为buildFilename会生成默认文件名

		// 确定文件扩展名
		ext := p.determineFileExtension(fileExtension, url)

		// 构建保存路径
		savePath := p.buildSavePath(filename, ext)

		task := DownloadTask{
			URL:      url,
			Filename: filename,
			SavePath: savePath,
			FileType: ext,
			RowIndex: i + 2, // Excel 行号（从1开始）
		}

		tasks = append(tasks, task)
	}

	if len(tasks) == 0 {
		return nil, fmt.Errorf("没有找到有效的下载任务")
	}

	return tasks, nil
}

func (p *ExcelParser) buildFilename(row []string, nameColIndices []int, separator string) string {
	var nameParts []string

	for _, colIndex := range nameColIndices {
		if colIndex < len(row) {
			value := strings.TrimSpace(row[colIndex])
			if value != "" {
				// 清理文件名中的非法字符
				value = p.cleanFilename(value)
				if value != "" { // 确保清理后不为空
					nameParts = append(nameParts, value)
				}
			}
		}
	}

	// 如果所有列都为空，生成默认文件名
	if len(nameParts) == 0 {
		// 使用时间戳和随机数生成唯一文件名
		timestamp := time.Now().Format("20060102_150405")
		return fmt.Sprintf("file_%s_%d", timestamp, time.Now().Nanosecond()%10000)
	}

	return strings.Join(nameParts, separator)
}

func (p *ExcelParser) cleanFilename(filename string) string {
	// 移除或替换文件名中的非法字符
	illegalChars := `<>:"/\|?*`
	for _, char := range illegalChars {
		filename = strings.ReplaceAll(filename, string(char), "_")
	}

	// 移除开头和结尾的空格和点
	filename = strings.Trim(filename, " .")

	// 限制长度
	if len(filename) > 200 {
		filename = filename[:200]
	}

	return filename
}

func (p *ExcelParser) determineFileExtension(configuredExt, url string) string {
	// 优先使用配置的扩展名
	if configuredExt != "" {
		return strings.TrimPrefix(configuredExt, ".")
	}

	// 从URL中提取扩展名
	if strings.Contains(url, ".") {
		parts := strings.Split(url, ".")
		if len(parts) > 1 {
			ext := strings.ToLower(parts[len(parts)-1])
			// 清理扩展名（移除查询参数等）
			if idx := strings.Index(ext, "?"); idx != -1 {
				ext = ext[:idx]
			}
			if idx := strings.Index(ext, "#"); idx != -1 {
				ext = ext[:idx]
			}
			// 只保留字母数字扩展名
			if len(ext) <= 6 && p.isAlphanumeric(ext) {
				return ext
			}
		}
	}

	return "bin" // 默认扩展名
}

func (p *ExcelParser) buildSavePath(filename, extension string) string {
	if !strings.Contains(filename, ".") && extension != "" {
		filename = filename + "." + extension
	}
	return filepath.Join(p.config.OutputDir, filename)
}

func (p *ExcelParser) columnToIndex(column string) int {
	column = strings.ToUpper(strings.TrimSpace(column))
	index := 0
	for _, char := range column {
		if char < 'A' || char > 'Z' {
			return -1
		}
		index = index*26 + int(char-'A') + 1
	}
	return index - 1 // 转换为0-based索引
}

func (p *ExcelParser) columnsToIndices(columns string) []int {
	parts := strings.Split(columns, ",")
	var indices []int

	for _, part := range parts {
		index := p.columnToIndex(strings.TrimSpace(part))
		if index >= 0 {
			indices = append(indices, index)
		}
	}

	return indices
}

func (p *ExcelParser) isAlphanumeric(s string) bool {
	for _, r := range s {
		if !unicode.IsLetter(r) && !unicode.IsNumber(r) {
			return false
		}
	}
	return true
}

// findColumnIndex 查找列索引（支持表头名称或列号）
func (p *ExcelParser) findColumnIndex(column string, headers []string) int {
	column = strings.TrimSpace(column)

	// 尝试通过表头名称查找
	for i, header := range headers {
		if strings.TrimSpace(header) == column {
			return i
		}
	}

	// 如果找不到，尝试作为列号（A, B, C...）解析
	return p.columnToIndex(column)
}

// findColumnsIndices 查找多个列的索引
func (p *ExcelParser) findColumnsIndices(columns string, headers []string) []int {
	parts := strings.Split(columns, ",")
	var indices []int

	for _, part := range parts {
		index := p.findColumnIndex(strings.TrimSpace(part), headers)
		if index >= 0 {
			indices = append(indices, index)
		}
	}

	return indices
}
