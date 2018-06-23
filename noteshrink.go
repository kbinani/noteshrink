package noteshrink

import (
	"image"
	"image/color"
	"math/rand"
)

type Option struct {
	SampleFraction      float32
	BrightnessThreshold float32
	SaturationThreshold float32
	NumColors           int
	KmeansMaxIter       int
	Saturate            bool
	WhiteBackground     bool
}

func MakeDefaultOption() *Option {
	return &Option{0.05, 0.25, 0.20, 8, 40, true, true}
}

func Shrink(input image.Image, option *Option) *image.RGBA {
	if option == nil {
		option = MakeDefaultOption()
	}
	img, rect := load(input)
	samples := samplePixels(img, *option)
	palette, origBgColor := createPalette(samples, *option)
	bgColor := origBgColor
	if option.Saturate {
		palette = saturatePalette(palette)
	}
	if option.WhiteBackground {
		bgColor = rgbf{255, 255, 255}
	}
	result := applyPalette(img, palette, origBgColor, bgColor, *option)
	shrinked := image.NewRGBA(rect)
	idx := 0
	for y := 0; y < rect.Dy(); y++ {
		for x := 0; x < rect.Dx(); x++ {
			c := result[idx]
			r := uint8(c[0])
			g := uint8(c[1])
			b := uint8(c[2])
			shrinked.SetRGBA(x, y, color.RGBA{r, g, b, 255})
			idx += 1
		}
	}
	return shrinked
}

func saturatePalette(palette []rgbf) []rgbf {
	result := []rgbf{}

	var maxSat float32 = 0
	var minSat float32 = 1
	for _, pal := range palette {
		_, s, _ := rgbToHsv(pal)
		maxSat = max(maxSat, s)
		minSat = min(minSat, s)
	}
	for _, pal := range palette {
		h, s, v := rgbToHsv(pal)
		newSat := (s - minSat) / (maxSat - minSat)
		saturated := hsvToRgb(h, newSat, v)
		result = append(result, saturated)
	}

	return result
}

func applyPalette(img []rgbf, palette []rgbf, origBgColor, bgColor rgbf, option Option) []rgbf {
	fgMask := createForegroundMask(origBgColor, img, option)
	result := []rgbf{}
	for i := 0; i < len(img); i++ {
		if !fgMask[i] {
			result = append(result, bgColor)
			continue
		}
		p := img[i]
		minidx := closest(p, palette)
		if minidx == 0 {
			result = append(result, bgColor)
		} else {
			result = append(result, palette[minidx])
		}
	}
	return result
}

func max(a, b float32) float32 {
	if a > b {
		return a
	} else {
		return b
	}
}

func min(a, b float32) float32 {
	if a > b {
		return b
	} else {
		return a
	}
}

func abs(a float32) float32 {
	if a < 0 {
		return -a
	} else {
		return a
	}
}

func createForegroundMask(bgColor rgbf, samples []rgbf, option Option) []bool {
	_, sBg, vBg := rgbToHsv(bgColor)
	sSamples := []float32{}
	vSamples := []float32{}
	for _, sample := range samples {
		_, s, v := rgbToHsv(sample)
		sSamples = append(sSamples, s)
		vSamples = append(vSamples, v)
	}

	result := []bool{}
	for i := 0; i < len(samples); i++ {
		sDiff := abs(sBg - sSamples[i])
		vDiff := abs(vBg - vSamples[i])
		fg := vDiff >= option.BrightnessThreshold || sDiff >= option.SaturationThreshold
		result = append(result, fg)
	}
	return result
}

func quantize(image []rgbf, bitsPerChannel uint8) []rgbf {
	shift := 8 - bitsPerChannel
	halfbin := uint8((1 << shift) >> 1)

	result := []rgbf{}

	for i := 0; i < len(image); i++ {
		var p rgbf
		for j := 0; j < 3; j++ {
			p[j] = float32((uint8(image[i][j])>>shift)<<shift + halfbin)
		}
		result = append(result, p)
	}
	return result
}

func findBackgroundColor(image []rgbf, bitsPerChannel uint8) rgbf {
	quantized := quantize(image, bitsPerChannel)
	count := make(map[rgbf]int)
	maxcount := 1
	maxvalue := quantized[0]
	for i := 1; i < len(quantized); i++ {
		v := quantized[i]
		c := count[v]
		c += 1
		if c > maxcount {
			maxcount = c
			maxvalue = v
		}
		count[v] = c
	}
	return maxvalue
}

func round(v float32) float32 {
	return float32(int(v + 0.5))
}

func createPalette(samples []rgbf, option Option) ([]rgbf, rgbf) {
	bgColor := findBackgroundColor(samples, 6)
	fgMask := createForegroundMask(bgColor, samples, option)
	data := []rgbf{}
	for i := 0; i < len(samples); i++ {
		if !fgMask[i] {
			continue
		}
		var v rgbf
		for j := 0; j < 3; j++ {
			v[j] = float32(samples[i][j])
		}
		data = append(data, v)
	}
	mean := kMeans(data, option.NumColors-1, option.KmeansMaxIter)
	palette := []rgbf{}
	palette = append(palette, bgColor)
	for i := 0; i < len(mean); i++ {
		c := mean[i]
		r := round(c[0])
		g := round(c[1])
		b := round(c[2])
		palette = append(palette, rgbf{r, g, b})
	}
	return palette, bgColor
}

func samplePixels(img []rgbf, option Option) []rgbf {
	numPixels := len(img)
	numSamples := int(float32(numPixels) * option.SampleFraction)
	shuffled := []rgbf{}
	for i := 0; i < len(img); i++ {
		shuffled = append(shuffled, img[i])
	}
	rand.Shuffle(len(shuffled), func(i, j int) {
		shuffled[i], shuffled[j] = shuffled[j], shuffled[i]
	})
	result := []rgbf{}
	for i := 0; i < numSamples; i++ {
		result = append(result, shuffled[i])
	}
	return result
}

type rgbf [3]float32

func load(img image.Image) ([]rgbf, image.Rectangle) {
	bounds := img.Bounds()
	result := []rgbf{}
	for y := 0; y < bounds.Dy(); y++ {
		for x := 0; x < bounds.Dx(); x++ {
			color := img.At(x, y)
			var p rgbf
			r, g, b, _ := color.RGBA()
			p[0] = float32(uint8(r))
			p[1] = float32(uint8(g))
			p[2] = float32(uint8(b))
			result = append(result, p)
		}
	}
	return result, img.Bounds()
}

func add(a, b rgbf) rgbf {
	var r rgbf
	for i := 0; i < 3; i++ {
		r[i] = a[i] + b[i]
	}
	return r
}

func mul(a rgbf, scalar float32) rgbf {
	var r rgbf
	for i := 0; i < 3; i++ {
		r[i] = a[i] * scalar
	}
	return r
}

func closest(p rgbf, means []rgbf) int {
	idx := 0
	minimum := squareDistance(p, means[0])
	for i := 1; i < len(means); i++ {
		squaredDistance := squareDistance(p, means[i])
		if squaredDistance < minimum {
			minimum = squaredDistance
			idx = i
		}
	}
	return idx
}

func kMeans(data []rgbf, k int, maxItr int) []rgbf {
	means := []rgbf{}
	for i := 0; i < k; i++ {
		h := float32(i) / float32(k-1)
		p := hsvToRgb(h, 1, 1)
		means = append(means, p)
	}

	clusters := make([]int, len(data))
	for i, d := range data {
		clusters[i] = closest(d, means)
	}

	mLen := make([]int, len(means))
	for itr := 0; itr < maxItr; itr++ {
		for i := range means {
			means[i] = rgbf{0, 0, 0}
			mLen[i] = 0
		}
		for i, p := range data {
			cluster := clusters[i]
			m := add(means[cluster], p)
			means[cluster] = m
			mLen[cluster]++
		}
		for i := range means {
			count := mLen[i]
			if count <= 0 {
				count = 1
			}
			m := mul(means[i], 1/float32(count))
			means[i] = m
		}
		var changes int
		for i, p := range data {
			if cluster := closest(p, means); cluster != clusters[i] {
				changes++
				clusters[i] = cluster
			}
		}
		if changes == 0 {
			break
		}
	}
	return means
}

func rgbToHsv(p rgbf) (h, s, v float32) {
	r := p[0] / 255
	g := p[1] / 255
	b := p[2] / 255
	max := max(max(r, g), b)
	min := min(min(r, g), b)
	h = max - min
	if h > 0 {
		if max == r {
			h = (g - b) / h
			if h < 0 {
				h += 6
			}
		} else if max == g {
			h = 2 + (b-r)/h
		} else {
			h = 4 + (r-g)/h
		}
	}
	h /= 6
	s = max - min
	if max > 0 {
		s /= max
	}
	v = max
	return h, s, v
}

func hsvToRgb(h, s, v float32) rgbf {
	r := v
	g := v
	b := v
	if s > 0 {
		h *= 6.
		i := int(h)
		f := h - float32(i)
		switch i {
		default:
		case 0:
			g *= 1 - s*(1-f)
			b *= 1 - s
		case 1:
			r *= 1 - s*f
			b *= 1 - s
		case 2:
			r *= 1 - s
			b *= 1 - s*(1-f)
		case 3:
			r *= 1 - s
			g *= 1 - s*f
		case 4:
			r *= 1 - s*(1-f)
			g *= 1 - s
		case 5:
			g *= 1 - s
			b *= 1 - s*f
		}
	}
	return rgbf{r * 255, g * 255, b * 255}
}

func squareDistance(a, b rgbf) float32 {
	var squareDistance float32 = 0
	for i := 0; i < 3; i++ {
		squareDistance += (a[i] - b[i]) * (a[i] - b[i])
	}
	return squareDistance
}
