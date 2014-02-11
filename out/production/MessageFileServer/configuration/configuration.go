package configuration

import (
	"strings"
)

import (
	"github.com/Unknwon/goconfig"
)

type Configuration struct {
	Port, RootPath, CommonFile, Image, Audio, PicExt, AudioExt string
}

var (
	Config *goconfig.ConfigFile
	Conf   *Configuration
)

func (conf *Configuration) IsPicType(extName string) bool {

	return strings.Contains(conf.PicExt, extName)
}

func (conf *Configuration) IsAudioType(extName string) bool {

	return strings.Contains(conf.AudioExt, extName)
}

func init() {
	Config, _ = goconfig.LoadConfigFile("config.ini")
	port, _ := Config.GetValue("base", "port")
	rootPath, _ := Config.GetValue("path", "rootPath")
	commonfile, _ := Config.GetValue("path", "commonfile")
	image, _ := Config.GetValue("path", "image")
	audio, _ := Config.GetValue("path", "audio")
	picExt, _ := Config.GetValue("ext", "picExt")
	audioExt, _ := Config.GetValue("ext", "audioExt")
	Conf = &Configuration{
		Port:       port,
		RootPath:   rootPath,
		CommonFile: commonfile,
		Image:      image,
		Audio:      audio,
		PicExt:     picExt,
		AudioExt:   audioExt,
	}
}
