//go:build ignore

// generate.go produces 7 placeholder 32×32px 1-bit PNG icons for the oioni UI.
// Run: go run generate.go (from cmd/oioni/ui/icons/ directory)
package main

import (
	"image"
	"image/color"
	"image/png"
	"os"
)

func main() {
	icons := map[string]func(*image.Gray){}

	// Config: gear-like cross with dots
	icons["config"] = func(img *image.Gray) {
		drawCross(img, 16, 16, 8)
		drawRing(img, 16, 16, 10, 12)
	}
	// System: 3 horizontal bars
	icons["system"] = func(img *image.Gray) {
		for _, y := range []int{8, 15, 22} {
			for x := 6; x < 26; x++ {
				img.SetGray(x, y, color.Gray{Y: 0})
				img.SetGray(x, y+1, color.Gray{Y: 0})
			}
		}
	}
	// Attack: lightning bolt
	icons["attack"] = func(img *image.Gray) {
		drawLine(img, 18, 4, 12, 16)
		drawLine(img, 12, 16, 20, 16)
		drawLine(img, 20, 16, 14, 28)
	}
	// DFIR: magnifying glass
	icons["dfir"] = func(img *image.Gray) {
		drawRing(img, 13, 13, 7, 9)
		drawLine(img, 18, 18, 26, 26)
	}
	// Info: circle with i
	icons["info"] = func(img *image.Gray) {
		drawRing(img, 16, 16, 11, 13)
		img.SetGray(16, 10, color.Gray{Y: 0})
		img.SetGray(16, 11, color.Gray{Y: 0})
		for y := 13; y <= 20; y++ {
			img.SetGray(16, y, color.Gray{Y: 0})
		}
	}
	// Oni: mask shape
	icons["oni"] = func(img *image.Gray) {
		for x := 8; x < 24; x++ {
			img.SetGray(x, 8, color.Gray{Y: 0})
			img.SetGray(x, 24, color.Gray{Y: 0})
		}
		for y := 8; y <= 24; y++ {
			img.SetGray(8, y, color.Gray{Y: 0})
			img.SetGray(23, y, color.Gray{Y: 0})
		}
		for y := 12; y <= 16; y++ {
			for x := 10; x <= 13; x++ {
				img.SetGray(x, y, color.Gray{Y: 255})
			}
			for x := 18; x <= 21; x++ {
				img.SetGray(x, y, color.Gray{Y: 255})
			}
		}
	}
	// Back: left chevron
	icons["back"] = func(img *image.Gray) {
		drawLine(img, 20, 6, 12, 16)
		drawLine(img, 12, 16, 20, 26)
	}

	for name, fn := range icons {
		img := image.NewGray(image.Rect(0, 0, 32, 32))
		for y := 0; y < 32; y++ {
			for x := 0; x < 32; x++ {
				img.SetGray(x, y, color.Gray{Y: 255})
			}
		}
		fn(img)
		f, err := os.Create(name + ".png")
		if err != nil {
			panic(err)
		}
		if err := png.Encode(f, img); err != nil {
			f.Close()
			panic(err)
		}
		f.Close()
	}
}

func drawCross(img *image.Gray, cx, cy, r int) {
	for i := -r; i <= r; i++ {
		img.SetGray(cx+i, cy, color.Gray{Y: 0})
		img.SetGray(cx, cy+i, color.Gray{Y: 0})
	}
}

func drawRing(img *image.Gray, cx, cy, r1, r2 int) {
	for y := -r2; y <= r2; y++ {
		for x := -r2; x <= r2; x++ {
			d2 := x*x + y*y
			if d2 >= r1*r1 && d2 <= r2*r2 {
				img.SetGray(cx+x, cy+y, color.Gray{Y: 0})
			}
		}
	}
}

func drawLine(img *image.Gray, x1, y1, x2, y2 int) {
	dx := x2 - x1
	dy := y2 - y1
	steps := dx
	if dy > dx {
		steps = dy
	}
	if steps < 0 {
		steps = -steps
	}
	if steps == 0 {
		img.SetGray(x1, y1, color.Gray{Y: 0})
		return
	}
	for i := 0; i <= steps; i++ {
		x := x1 + i*dx/steps
		y := y1 + i*dy/steps
		img.SetGray(x, y, color.Gray{Y: 0})
	}
}
