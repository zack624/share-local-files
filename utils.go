package main

import (
	"log"
	"net/http"
	"os"
	"os/exec"
	"time"
)

// 渲染HTML模板公共函数
func renderTemplate(w http.ResponseWriter, templateName string, data any) {
	err := templates.ExecuteTemplate(w, templateName, data)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

// 从本地文件加载数据到内存，CopyText
func loadData(path string) ([]byte, error) {
	bytes, err := os.ReadFile(path)
	if err != nil {
		log.Fatal("Open file failed!")
		return nil, err
	}
	return bytes, nil
}

// 内存保存数据到本地文件
func saveData(path string, data []byte) error {
	err := os.WriteFile(path, data, 0600)
	if err != nil {
		log.Fatal("Write file failed!")
	}
	return err
}

// 起新线程调用浏览器
// 在http.ListenAndServe之后运行
func openBrowser(url string) {
	time.Sleep(3 * time.Second)
	exec.Command("cmd", "/c", "start", url).Start()
}
