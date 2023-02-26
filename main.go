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
	"archive/zip"
	"fmt"
	"html/template"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"path"
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
	fmt.Println("文件传输助手")
	// 读取当前机器IP，打印网址
	ip := "localhost"
	conn, err := net.Dial("udp", "8.8.8.8:53")
	if err != nil {
		log.Fatal(err)
	}
	localAddr := conn.LocalAddr().(*net.UDPAddr)
	ip = strings.Split(localAddr.String(), ":")[0]
	log.Printf("本机IP：%s", ip)
	fmt.Printf("请在电脑端和手机浏览器打开网址：http://%s", ip)
	// 添加路由，绑定url和HandlerFunc
	log.Print("添加路由：/, /uploadFile, /uploadCopy, /downloadFile, /deleteFile")
	http.HandleFunc("/", indexHandler)
	http.HandleFunc("/uploadFile", uploadFileHandler)
	http.HandleFunc("/uploadCopy", uploadCopyTextHandler)
	http.HandleFunc("/downloadFile/", downloadFileHandler)
	http.HandleFunc("/downloadSelectedFiles", downloadSelectedFilesHandler)
	http.HandleFunc("/deleteFile/", deleteFileHandler)
	http.HandleFunc("/deleteFiles", deleteFilesHandler)
	// 调用浏览器打开网址
	go openBrowser("http://" + ip)
	log.Fatal(http.ListenAndServe(ip+":80", nil))
}

// 主页，刷新加载data内存
func indexHandler(w http.ResponseWriter, r *http.Request) {
	log.Print("请求：/")
	// 扫描本地目录下的所有文件，初始化files
	log.Printf("扫描目录：%s", dir)
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
	log.Print("请求：/uploadFile")
	err := r.ParseMultipartForm(500 << 20) // FIXME 限制上传文件大小500MB，但是不生效？
	if err != nil {
		log.Fatal("ParseMultipartForm failed!")
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
	// FIXME 未选择文件就点击上传会报错
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
	log.Printf("上传文件：%s，文件大小：%d", fileName, len)
	// 返回index
	http.Redirect(w, r, "/", http.StatusFound)
}

// 接收需要复制的文本
// 复制文本时，直接修改存储文本文件的内容，返回index.html
func uploadCopyTextHandler(w http.ResponseWriter, r *http.Request) {
	// 获取请求的textCopy参数
	textCopy := r.FormValue("textCopy")
	// 持久化copyStorage文件
	saveData(copyStorage, []byte(textCopy))
	// 返回index
	http.Redirect(w, r, "/", http.StatusFound)
}

// 下载文件
// 请求URL模式：/downloadFile/filename
func downloadFileHandler(w http.ResponseWriter, r *http.Request) {
	log.Printf("请求：%s", r.URL.Path)
	fileName := r.URL.Path[len("/downloadFile/"):]
	file, err := os.ReadFile(dir + fileName)
	if err != nil {
		log.Fatal("Read file failed!")
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
	w.Write(file)
	log.Printf("下载文件：%s", fileName)
}

// 下载勾选文件
// 把勾选文件打包成ZIP包，返回
// 如果只勾选一个文件，则直接返回该文件
func downloadSelectedFilesHandler(w http.ResponseWriter, r *http.Request) {
	log.Printf("请求：%s", r.URL.Path)
	r.ParseForm()
	files := r.Form["selectedFile"]
	// 设置Content-Type为下载二进制文件
	w.Header().Set("Content-Type", "application/octet-stream")
	if len(files) == 1 {
		fileName := files[0]
		file, err := os.ReadFile(dir + fileName)
		if err != nil {
			log.Fatal("Read file failed!")
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
		w.Header().Set("Content-Disposition", "attachment; filename="+fileName)
		w.Write(file)
		log.Printf("批量下载文件：%s", fileName)
	} else {
		zipTempFile := "批量下载.zip"
		zipFile, err := os.Create(zipTempFile)
		if err != nil {
			log.Fatal("Create zip file failed!")
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
		zipw := zip.NewWriter(zipFile)
		for _, fileName := range files {
			file, err := os.Open(dir + fileName)
			if err != nil {
				log.Fatal("Create file failed!")
				http.Error(w, err.Error(), http.StatusInternalServerError)
			}
			wr, err := zipw.Create(fileName)
			if err != nil {
				log.Fatal("Failed to create entry in zip file!")
				http.Error(w, err.Error(), http.StatusInternalServerError)
			}
			if _, err := io.Copy(wr, file); err != nil {
				log.Fatal("Failed to write to zip!")
				http.Error(w, err.Error(), http.StatusInternalServerError)
			}
			file.Close()
		}
		zipw.Close()
		w.Header().Set("Content-Disposition", "attachment; filename="+zipTempFile)
		zipFileStorage, _ := os.ReadFile(zipTempFile)
		w.Write(zipFileStorage)
		log.Printf("批量下载文件压缩包：%s", zipTempFile)
		// TODO 删除ZIP临时文件
	}
}

// 删除文件
// 请求URL模式：/deleteFile/filename
func deleteFileHandler(w http.ResponseWriter, r *http.Request) {
	log.Printf("请求：%s", r.URL.Path)
	fileName := r.URL.Path[len("/deleteFile/"):]
	err := os.Remove(dir + fileName)
	log.Printf("删除文件：%s", fileName)
	if err != nil {
		log.Fatal("Remove file failed!")
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
	http.Redirect(w, r, "/", http.StatusFound)
}

// 清空文件列表
// 请求URL模式：/deleteFiles
func deleteFilesHandler(w http.ResponseWriter, r *http.Request) {
	// 删除working_dir下所有文件
	log.Printf("请求：%s", r.URL.Path)
	files, _ := os.ReadDir(dir)
	var err error
	for _, v := range files {
		fileName := path.Join(dir, v.Name())
		err = os.Remove(fileName)
		log.Printf("删除文件：%s", fileName)
	}
	if err != nil {
		log.Fatal("Remove all files failed!")
		// FIXME 错误跳转到404网页
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
	http.Redirect(w, r, "/", http.StatusFound)
}
