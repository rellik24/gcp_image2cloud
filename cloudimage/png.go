package cloudimage

import (
	"fmt"
	"image/png"
	"os"

	"github.com/nfnt/resize"
)

// compressPNG :
func compressPNG(filename string) error {
	newFilename := fmt.Sprintf("%st_%s", DirPath, filename)
	if err := os.Rename(fmt.Sprintf("%s%s", DirPath, filename), newFilename); err != nil {
		return err
	}

	infile, err := os.Open(filename)
	if err != nil {
		return err
	}
	defer infile.Close()

	img, err := png.Decode(infile)
	if err != nil {
		return err
	}

	// 尺寸減半
	img = resize.Resize(uint(img.Bounds().Dx()/2), uint(img.Bounds().Dy()/2), img, resize.Lanczos3)

	outfile, err := os.Create(fmt.Sprintf("%s%s", DirPath, filename))
	if err != nil {
		return err
	}
	defer outfile.Close()

	// 儲存到指定路径
	err = png.Encode(outfile, img)
	if err != nil {
		return err
	}

	os.Remove(newFilename)
	return nil
}
