package main

import (
	"batch-downloader/gui"
	"log"
)

func main() {
	app := gui.NewApp()
	if err := app.Run(); err != nil {
		log.Fatal("应用启动失败: ", err)
	}
}
