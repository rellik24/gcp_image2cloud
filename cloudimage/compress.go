package cloudimage

import (
	"errors"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"os"
)

// 暫存位置
var DirPath string = ".tmp/"

// Compress :
func Compress(filename string) error {
	file, err := os.Open(filename)
	if err != nil {
		return err
	}
	defer file.Close()

	_, format, err := image.DecodeConfig(file)
	if err != nil {
		return err
	}

	if format == "png" {
		return compressPNG(filename)
	} else if format == "jpeg" {
		return compressJPG(filename)
	} else {
		return errors.New("unknown file format")
	}
}
