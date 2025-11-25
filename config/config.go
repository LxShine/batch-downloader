package config

import (
	"os"
	"path/filepath"
)

type Config struct {
	ExcelPath      string `json:"excel_path"`
	OutputDir      string `json:"output_dir"`
	MaxConcurrency int    `json:"max_concurrency"`
	Timeout        int    `json:"timeout"`
	RetryCount     int    `json:"retry_count"`
}

func NewConfig() *Config {
	currentDir, _ := os.Getwd()
	downloadsDir := filepath.Join(currentDir, "downloads")

	// 创建下载目录
	os.MkdirAll(downloadsDir, 0755)

	return &Config{
		ExcelPath:      "",
		OutputDir:      downloadsDir,
		MaxConcurrency: 10,
		Timeout:        30,
		RetryCount:     3,
	}
}
