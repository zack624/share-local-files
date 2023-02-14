package main

// 功能：支持局域网下共享文件或复制文本
// 开发过程大纲：
// 0.设计基础的数据结构：File，有属性name，time，size，全局，只保存在内存中
// 1.设计前后端传送的数据结构：Body，有属性CopyText，Files，包括复制文本内容和文件列表
// 2.编写HTML模板，解析，渲染
// 3.读取、修改本地目录下文件列表
// 4.HTML里提供文件上传下载功能，文本和文件统一处理
// 5.读写本地文件，copyText数据写入磁盘
// 6.读取当前机器IP，打印网址

import (
	"fmt"
	"html/template"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"time"
)

type file struct {
	Name string
	Size int64
	Time time.Time
}

// 设计前后端传送的数据结构
type body struct {
	Files    []file
	CopyText []byte
}

var (
	data body
	// 定义存储目录，和复制内容的特定文本文件名
	dir         = "working_dir/"
	copyStorage = "copyStorage.txt"
	// 统一解析HTML模板，只需要一次
	templates = template.Must(template.ParseFiles("index.html"))
)

func main() {
	fmt.Println("电脑手机互传工具")
	// 读取当前机器IP，打印网址
	ip := "localhost"
	conn, err := net.Dial("udp", "8.8.8.8:53")
	if err != nil {
		log.Fatal(err)
	}
	localAddr := conn.LocalAddr().(*net.UDPAddr)
	ip = strings.Split(localAddr.String(), ":")[0]
	//log.Printf("本机IP：%s", ip)
	fmt.Printf("请在电脑端和手机浏览器打开网址：http://%s", ip)
	// 添加路由，绑定url和HandlerFunc
	//log.Print("添加路由：/, /uploadFile, /uploadCopy, /downloadFile, /deleteFile")
	http.HandleFunc("/", indexHandler)
	http.HandleFunc("/uploadFile", uploadFileHandler)
	http.HandleFunc("/uploadCopy", uploadCopyTextHandler)
	http.HandleFunc("/downloadFile/", downloadFileHandler)
	http.HandleFunc("/deleteFile/", deleteFileHandler)
	// 调用浏览器打开网址
	go openBrowser("http://" + ip)
	log.Fatal(http.ListenAndServe(ip+":80", nil))
}

// 主页，刷新加载data内存
func indexHandler(w http.ResponseWriter, r *http.Request) {
	//log.Print("请求：/")
	// 扫描本地目录下的所有文件，初始化files
	//log.Printf("扫描目录：%s", dir)
	c, err := os.ReadDir(dir)
	if err != nil {
		panic(err)
	}
	var files []file
	for _, entry := range c {
		info, _ := entry.Info()
		name := info.Name()
		size := info.Size()
		time := info.ModTime()
		file := file{Name: name, Size: size, Time: time}
		files = append(files, file)
	}
	// 获取CopyText数据
	copyText, _ := loadData(copyStorage)
	data = body{Files: files, CopyText: copyText}
	renderTemplate(w, "index.html", data)
}

// 接收上传的文件，修改本地目录内容
// 上传文件时，直接添加在目录中，返回index.html
func uploadFileHandler(w http.ResponseWriter, r *http.Request) {
	// 解析POST请求，enctype="multipart/form-data"，得到文件流
	//log.Print("请求：/uploadFile")
	err := r.ParseMultipartForm(500 << 20) // FIXME 限制上传文件大小500MB，但是不生效？
	if err != nil {
		log.Fatal("ParseMultipartForm failed!")
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
	file, fileHeader, err := r.FormFile("fileUpload")
	if err != nil {
		log.Fatal("Http request Form file err!")
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
	defer file.Close() // defer语法会在函数返回前执行，类似于finally语法
	// 文件流持久化到本地
	fileName := dir + fileHeader.Filename
	localFile, err := os.Create(fileName)
	if err != nil {
		log.Fatal("Create file failed!")
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
	defer localFile.Close()
	len, err := io.Copy(localFile, file)
	if err != nil {
		log.Fatal("Copy file failed!")
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
	_ = len
	//log.Printf("上传文件：%s，文件大小：%d", fileName, len)
	// 返回index
	http.Redirect(w, r, "/", http.StatusFound)
}

// 接收需要复制的文本
// 复制文本时，直接修改存储文本文件的内容，返回index.html
func uploadCopyTextHandler(w http.ResponseWriter, r *http.Request) {
	// 获取请求的textCopy参数
	//r.ParseForm()
	textCopy := r.FormValue("textCopy")
	// 持久化copyStorage文件
	saveData(copyStorage, []byte(textCopy))
	// 返回index
	http.Redirect(w, r, "/", http.StatusFound)
}

// 下载文件
// 请求URL模式：/downloadFile/filename
func downloadFileHandler(w http.ResponseWriter, r *http.Request) {
	//log.Printf("请求：%s", r.URL.Path)
	fileName := r.URL.Path[len("/downloadFile/"):]
	file, err := os.ReadFile(dir + fileName)
	if err != nil {
		log.Fatal("Read file failed!")
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
	w.Write(file)
}

// 删除文件
// 请求URL模式：/deleteFile/filename
func deleteFileHandler(w http.ResponseWriter, r *http.Request) {
	//log.Printf("请求：%s", r.URL.Path)
	fileName := r.URL.Path[len("/deleteFile/"):]
	err := os.Remove(dir + fileName)
	if err != nil {
		log.Fatal("Remove file failed!")
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
	http.Redirect(w, r, "/", http.StatusFound)
}

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
