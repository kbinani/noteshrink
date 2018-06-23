package main


import (
	"image/png"
	"os"
	"github.com/kbinani/noteshrink"
)


func main() {
	input := os.Args[1]
	output := "shrinked_" + input
	in, err := os.Open(input)
	if err != nil {
		panic(err)
	}
	defer in.Close()
	img, err := png.Decode(in)
	if err != nil {
		panic(err)
	}
	shrinked := noteshrink.Shrink(img, nil)

	out, err := os.Create(output)
	if err != nil {
		panic(err)
	}
	defer out.Close()
	png.Encode(out, shrinked)
}
