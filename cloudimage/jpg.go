package cloudimage

import (
	"fmt"
	"image/jpeg"
	"log"
	"os"

	"github.com/nfnt/resize"
)

var dirPath string = "./.tmp/"

// CompressJPG :
func CompressJPG(filename string) {
	file, err := os.Open(filename)
	if err != nil {
		log.Fatal(err)
	}
	defer file.Close()

	img, err := jpeg.Decode(file)
	if err != nil {
		log.Fatal(err)
	}

	// 尺寸減半
	img = resize.Resize(uint(img.Bounds().Dx()/2), uint(img.Bounds().Dy()/2), img, resize.Lanczos3)

	// 判断目录是否存在
	if _, err := os.Stat(dirPath); os.IsNotExist(err) {
		// 目录不存在，创建该目录
		err = os.Mkdir(dirPath, 0755)
		if err != nil {
			fmt.Println(err)
			return
		}
	}
	outfile, err := os.Create(fmt.Sprintf("%s/%s", dirPath, filename))
	if err != nil {
		log.Fatal(err)
	}
	defer outfile.Close()

	options := &jpeg.Options{Quality: 90}
	err = jpeg.Encode(outfile, img, options)
	if err != nil {
		log.Fatal(err)
	}

}
