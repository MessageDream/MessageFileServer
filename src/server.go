package main

import (
	. "./common"
	. "./configuration"
	"./filecache"
	"./handler"
)

import (
	"strconv"
)

import (
	"github.com/astaxie/beego"
)

func init() {
	FileCached = filecache.NewDefaultCache()
	err := FileCached.Start()
	CheckError(err)
}

func main() {
	defer FileCached.Stop()

	beego.SetStaticPath("/static", Conf.RootPath+"/"+Conf.CommonFile+"/js")
	beego.HttpPort, _ = strconv.Atoi(Conf.Port)

	beego.Router("/fileserver/uploadFile", &handler.UploadHandler{})
	beego.Router("/images/:id([0-9a-zA-Z./_]+)", &handler.GetImageHandler{})
	beego.Run()
}
