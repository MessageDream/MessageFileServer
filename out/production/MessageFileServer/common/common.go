package common

import (
	"../filecache"
	"../imageresize"
)

import (
	"bytes"
	"fmt"
	"image"
	"image/jpeg"
	"io"
	"os"
	"strings"
)

var (
	FileCached *filecache.FileCache
)

func CheckError(err error) bool {
	if err != nil {
		fmt.Println(err)
		return false
	}
	return true
}

//生成新图片
func ProducedNewPic(f io.Reader, imgX, imgY int) []byte {
	img, err := jpeg.Decode(f)
	if !CheckError(err) {
		panic(err)
	}
	rec := img.Bounds()

	var newImg image.Image
	if rec.Dx() > 2*imgX || rec.Dy() > 2*imgY {
		w, h := imgX, imgY
		if rec.Dx() > rec.Dy() {
			h = rec.Dy() * w / rec.Dx()
		} else {
			w = rec.Dx() * h / rec.Dy()
		}
		newImg = imageresize.Resample(img, rec, w, h)
	} else {
		newImg = imageresize.Resample(img, rec, rec.Dx(), rec.Dy())
	}
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, newImg, nil); !CheckError(err) {
		panic(err)
	}
	return buf.Bytes()
}

//获取扩展名
func GetExt(fileName string) string {
	return strings.Split(fileName, ".")[1]
}

//检查文件夹是否存在
func IsDirExists(path string) bool {
	fi, err := os.Stat(path)
	if err != nil {
		return os.IsExist(err)
	} else {
		return fi.IsDir()
	}
	//panic("not reached")
}

//检查文件是否存在
func IsFileExists(path string) bool {
	_, err := os.Stat(path)
	if err != nil {
		return os.IsExist(err)
	}
	return true
	//panic("not reached")
}
