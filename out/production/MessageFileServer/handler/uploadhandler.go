package handler

import (
	. "../common"
	. "../configuration"
)

import (
	"fmt"
	"io"
	"mime/multipart"
	//"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

const (
	EMPTYSTR   = ""
	CITYPATH   = "js/city"
	MOBILEPATH = "mobile"
	COMMONDIR  = "original"
)

type UploadHandler struct {
	BaseHandler
}

type responseStruct struct {
	Error                int
	Url, ClientInputName string
}

func getUnixNano() string {
	return strconv.FormatInt(time.Now().UnixNano(), 10)
}

//保存文件
func SaveFile(name, filePath, channel string, file multipart.File) (bool, string) {
	filePath = Conf.RootPath + "/" + filePath
	if !IsDirExists(filePath) {
		err := os.MkdirAll(filePath, (os.FileMode)(0666))
		if err != nil {
			fmt.Println(err)
			return false, EMPTYSTR
		}
	}

	f, er := os.OpenFile(filePath+"/"+name, os.O_WRONLY|os.O_CREATE, (os.FileMode)(0666))
	if er != nil {
		fmt.Println(er)
		return false, EMPTYSTR
	}
	defer f.Close()
	_, erro := io.Copy(f, file)
	if erro != nil {
		fmt.Println(erro)
		return false, EMPTYSTR
	}
	return true, channel + "/" + name
}

//处理文件
func FileHandler(name, channel string, file multipart.File) (bool, string) {

	if name == EMPTYSTR || channel == EMPTYSTR {
		return false, EMPTYSTR
	}

	//上传JS或css文件
	if strings.EqualFold(CITYPATH, channel) || strings.Contains(channel, MOBILEPATH) {
		filePath := Conf.CommonFile + "/" + channel
		return SaveFile(name, filePath, channel, file)
	}

	ext := GetExt(name)

	pathDir := channel + "/" + COMMONDIR + "/" + time.Now().Format("20060102")

	//上传图片和音频
	if Conf.IsPicType(ext) {
		pathDir := Conf.Image + "/" + pathDir
		channel = pathDir
		name = getUnixNano() + "." + ext
		return SaveFile(name, pathDir, channel, file)
	} else if Conf.IsAudioType(ext) {
		pathDir := Conf.Audio + "/" + pathDir
		channel = pathDir
		name = getUnixNano() + "." + ext
		return SaveFile(name, pathDir, channel, file)
	}
	return false, EMPTYSTR
}

// 处理uploadFile逻辑
func (self *UploadHandler) Post() {
	self.Ctx.Request.ParseMultipartForm(32 << 20)
	channel := self.Ctx.Request.FormValue("fileChannel")
	callBackUrl := self.Ctx.Request.FormValue("callBackUrl")

	if channel == EMPTYSTR || callBackUrl == EMPTYSTR {
		return
	}

	responseData := make([]*responseStruct, 0)

	for _, v := range self.Ctx.Request.MultipartForm.File {
		for _, f := range v {
			file, err := f.Open()
			if err != nil {
				fmt.Println(err)
				return
			}
			defer file.Close()
			if ok, url := FileHandler(f.Filename, channel, file); ok {
				responseData = append(responseData, &responseStruct{Error: 0, Url: url, ClientInputName: f.Filename})
			} else {
				responseData = append(responseData, &responseStruct{Error: 1, Url: url, ClientInputName: f.Filename})
			}
		}
	}
	self.Data["json"] = &responseData
	self.ServeJson()
}
