package color

import (
	"image"
	"image/color"
	"math"
)

type RGB struct {
	R, G, B uint8
}

func (c RGB) ToRGBA() color.RGBA {
	return color.RGBA{R: c.R, G: c.G, B: c.B, A: 255}
}

func (c RGB) Distance(other RGB) float64 {
	rDiff := float64(c.R) - float64(other.R)
	gDiff := float64(c.G) - float64(other.G)
	bDiff := float64(c.B) - float64(other.B)
	return math.Sqrt(rDiff*rDiff + gDiff*gDiff + bDiff*bDiff)
}

type ColorBox struct {
	Colors []RGB
	RMin, RMax int
	GMin, GMax int
	BMin, BMax int
}

func NewColorBox(colors []RGB) *ColorBox {
	box := &ColorBox{Colors: colors}
	if len(colors) == 0 {
		return box
	}
	box.RMin = 255
	box.GMin = 255
	box.BMin = 255
	for _, c := range colors {
		if int(c.R) < box.RMin {
			box.RMin = int(c.R)
		}
		if int(c.R) > box.RMax {
			box.RMax = int(c.R)
		}
		if int(c.G) < box.GMin {
			box.GMin = int(c.G)
		}
		if int(c.G) > box.GMax {
			box.GMax = int(c.G)
		}
		if int(c.B) < box.BMin {
			box.BMin = int(c.B)
		}
		if int(c.B) > box.BMax {
			box.BMax = int(c.B)
		}
	}
	return box
}

func (b *ColorBox) Volume() int {
	return (b.RMax - b.RMin + 1) * (b.GMax - b.GMin + 1) * (b.BMax - b.BMin + 1)
}

func (b *ColorBox) LongestSide() (string, int) {
	rRange := b.RMax - b.RMin
	gRange := b.GMax - b.GMin
	bRange := b.BMax - b.BMin
	
	maxRange := rRange
	side := "R"
	if gRange > maxRange {
		maxRange = gRange
		side = "G"
	}
	if bRange > maxRange {
		maxRange = bRange
		side = "B"
	}
	return side, maxRange
}

func (b *ColorBox) Split() (*ColorBox, *ColorBox) {
	if len(b.Colors) == 0 {
		return nil, nil
	}
	
	side, _ := b.LongestSide()
	
	switch side {
	case "R":
		b.sortByR()
	case "G":
		b.sortByG()
	case "B":
		b.sortByB()
	}
	
	mid := len(b.Colors) / 2
	box1 := NewColorBox(b.Colors[:mid])
	box2 := NewColorBox(b.Colors[mid:])
	return box1, box2
}

func (b *ColorBox) sortByR() {
	n := len(b.Colors)
	for i := 0; i < n-1; i++ {
		for j := 0; j < n-i-1; j++ {
			if b.Colors[j].R > b.Colors[j+1].R {
				b.Colors[j], b.Colors[j+1] = b.Colors[j+1], b.Colors[j]
			}
		}
	}
}

func (b *ColorBox) sortByG() {
	n := len(b.Colors)
	for i := 0; i < n-1; i++ {
		for j := 0; j < n-i-1; j++ {
			if b.Colors[j].G > b.Colors[j+1].G {
				b.Colors[j], b.Colors[j+1] = b.Colors[j+1], b.Colors[j]
			}
		}
	}
}

func (b *ColorBox) sortByB() {
	n := len(b.Colors)
	for i := 0; i < n-1; i++ {
		for j := 0; j < n-i-1; j++ {
			if b.Colors[j].B > b.Colors[j+1].B {
				b.Colors[j], b.Colors[j+1] = b.Colors[j+1], b.Colors[j]
			}
		}
	}
}

func (b *ColorBox) Average() RGB {
	if len(b.Colors) == 0 {
		return RGB{R: 0, G: 0, B: 0}
	}
	var r, g, bsum uint32
	for _, c := range b.Colors {
		r += uint32(c.R)
		g += uint32(c.G)
		bsum += uint32(c.B)
	}
	n := uint32(len(b.Colors))
	return RGB{
		R: uint8(r / n),
		G: uint8(g / n),
		B: uint8(bsum / n),
	}
}

func ExtractDominantColor(img image.Image, sampleHeightRatio float64) RGB {
	bounds := img.Bounds()
	width := bounds.Dx()
	height := bounds.Dy()
	
	sampleHeight := int(float64(height) * sampleHeightRatio)
	if sampleHeight < 1 {
		sampleHeight = 1
	}
	if sampleHeight > height {
		sampleHeight = height
	}
	
	colors := make([]RGB, 0, width*sampleHeight)
	
	sampleY := bounds.Max.Y - sampleHeight
	for y := sampleY; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			r, g, b, _ := img.At(x, y).RGBA()
			colors = append(colors, RGB{
				R: uint8(r >> 8),
				G: uint8(g >> 8),
				B: uint8(b >> 8),
			})
		}
	}
	
	if len(colors) == 0 {
		return RGB{R: 0, G: 0, B: 0}
	}
	
	return medianCut(colors, 1)[0]
}

func medianCut(colors []RGB, maxColors int) []RGB {
	if len(colors) == 0 || maxColors <= 0 {
		return []RGB{}
	}
	
	if len(colors) <= maxColors {
		avgBox := NewColorBox(colors)
		return []RGB{avgBox.Average()}
	}
	
	priorityQueue := make([]*ColorBox, 0, maxColors*2)
	initialBox := NewColorBox(colors)
	priorityQueue = append(priorityQueue, initialBox)
	
	for len(priorityQueue) < maxColors {
		if len(priorityQueue) == 0 {
			break
		}
		
		maxIdx := 0
		maxVol := priorityQueue[0].Volume()
		for i := 1; i < len(priorityQueue); i++ {
			if priorityQueue[i].Volume() > maxVol {
				maxVol = priorityQueue[i].Volume()
				maxIdx = i
			}
		}
		
		box := priorityQueue[maxIdx]
		priorityQueue = append(priorityQueue[:maxIdx], priorityQueue[maxIdx+1:]...)
		
		box1, box2 := box.Split()
		if box1 != nil && len(box1.Colors) > 0 {
			priorityQueue = append(priorityQueue, box1)
		}
		if box2 != nil && len(box2.Colors) > 0 {
			priorityQueue = append(priorityQueue, box2)
		}
	}
	
	result := make([]RGB, 0, len(priorityQueue))
	for _, box := range priorityQueue {
		result = append(result, box.Average())
	}
	
	return result
}
