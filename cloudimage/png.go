package cloudimage

import (
	"fmt"
	"image/png"
	"log"
	"os"

	"github.com/nfnt/resize"
)

// CompressPNG :
func CompressPNG(filename string) {
	infile, err := os.Open(filename)
	if err != nil {
		log.Fatal(err)
	}
	defer infile.Close()

	img, err := png.Decode(infile)
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

	// 儲存到指定路径
	err = png.Encode(outfile, img)
	if err != nil {
		log.Fatal(err)
	}

}
