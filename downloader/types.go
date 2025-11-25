package downloader

import "time"

type DownloadTask struct {
	URL      string `json:"url"`
	Filename string `json:"filename"`
	SavePath string `json:"save_path"`
	FileType string `json:"file_type"`
	RowIndex int    `json:"row_index"`
}

type DownloadResult struct {
	Task     DownloadTask  `json:"task"`
	Success  bool          `json:"success"`
	FileSize int64         `json:"file_size"`
	Error    error         `json:"error"`
	Duration time.Duration `json:"duration"`
	Filename string        `json:"filename"`
}
