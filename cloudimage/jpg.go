package cloudimage

import (
	"fmt"
	"image/jpeg"
	"os"

	"github.com/nfnt/resize"
)

// compressJPG :
func compressJPG(filename string) error {
	file, err := os.Open(filename)
	if err != nil {
		return err
	}
	defer file.Close()

	img, err := jpeg.Decode(file)
	if err != nil {
		return err
	}

	// 尺寸減半
	img = resize.Resize(uint(img.Bounds().Dx()/2), uint(img.Bounds().Dy()/2), img, resize.Lanczos3)

	// 判断目录是否存在
	if _, err := os.Stat(DirPath); os.IsNotExist(err) {
		// 目录不存在，创建该目录
		err = os.Mkdir(DirPath, 0755)
		if err != nil {
			return err
		}
	}
	outfile, err := os.Create(fmt.Sprintf("%s%s", DirPath, filename))
	if err != nil {
		return err
	}
	defer outfile.Close()

	options := &jpeg.Options{Quality: 90}
	err = jpeg.Encode(outfile, img, options)
	if err != nil {
		return err
	}
	return nil
}
