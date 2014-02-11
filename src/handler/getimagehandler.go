package handler

import (
	. "../common"
	. "../configuration"
	"../httpfileserver"
)

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"path"
	"strconv"
	"strings"
	"time"
)

type GetImageHandler struct {
	BaseHandler
}

const (
	SIZEDIC  = "htumbnail"
	ORIGINAL = "original"
)

//缓存文件
func CacheFile(filePath string) {
	er := FileCached.CacheNow(filePath)
	CheckError(er)
	fmt.Println("cached ok")
	fmt.Println(FileCached.Size())
}

//保存并缓存文件
func SaveFileAndCached(filePath string, data []byte) {
	dir, _ := path.Split(filePath)
	if !IsDirExists(dir) {
		os.MkdirAll(dir, (os.FileMode)(0666))
	}
	err := ioutil.WriteFile(filePath, data, (os.FileMode)(0644))
	if !CheckError(err) {
		return
	}
	CacheFile(filePath)
}

func (self *GetImageHandler) Get() {
	path := self.Ctx.Request.URL.Path
	absPath := Conf.RootPath + path
	dicFile := strings.Split(path, "/")

	contentBuf := bytes.NewBuffer(make([]byte, 0))

	n := len(dicFile)
	fName := dicFile[n-1]
	//缓存中获取
	itemContent, ok := FileCached.GetItem(absPath)
	if ok {
		contentBuf.Write(itemContent)
	} else if IsFileExists(absPath) {
		http.ServeFile(self.Ctx.ResponseWriter, self.Ctx.Request, absPath)
		go CacheFile(absPath)
		return
	} else if strings.Contains(path, SIZEDIC) { //如果获取指定尺寸的图片
		//寻找原图生成缩略图
		fSize := dicFile[n-3]
		wh := strings.Split(fSize, "_")
		imgX, _ := strconv.Atoi(wh[0])
		imgY, _ := strconv.Atoi(wh[1])

		path = strings.Replace(path, "/"+fSize, "", -1)
		ordi := Conf.RootPath + strings.Replace(path, SIZEDIC, ORIGINAL, -1)

		var buf []byte
		if item, y := FileCached.GetItem(ordi); y { //从缓存中获取原图
			reader := bytes.NewReader(item)
			buf = ProducedNewPic(reader, imgX, imgY)
		} else { //从磁盘中取
			f, err := os.Open(ordi)
			defer f.Close()
			if !CheckError(err) {
				return
			}
			buf = ProducedNewPic(f, imgX, imgY)
			go CacheFile(ordi) //缓存原图
		}

		_, er := contentBuf.Write(buf)
		if !CheckError(er) {
			return
		}

		go SaveFileAndCached(absPath, contentBuf.Bytes())
	} else {
		http.NotFound(self.Ctx.ResponseWriter, self.Ctx.Request)
		return
	}
	httpfileserver.ServeContent(self.Ctx.ResponseWriter, self.Ctx.Request, fName, time.Now(), int64(contentBuf.Len()), contentBuf)
}
