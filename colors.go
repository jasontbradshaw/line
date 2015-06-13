package main

import (
	"fmt"
	"math"
)

// used to reset the color value to default
const COLOR_ESCAPE = "\x1b"
const COLOR_RESET = COLOR_ESCAPE + "[0m"
const COLOR_CODE_FOREGROUND = 38
const COLOR_CODE_BACKGROUND = 48

// standard colors
const (
	COLOR_BLACK   = 0
	COLOR_RED     = 1
	COLOR_GREEN   = 2
	COLOR_YELLOW  = 3
	COLOR_BLUE    = 4
	COLOR_MAGENTA = 5
	COLOR_CYAN    = 6
	COLOR_WHITE   = 7
)

// converts an RGB color to hex
func rgbToHex(r, g, b int) int {
	return (r << 16) | (g << 8) | b
}

// converts a hex color to RGB
func hexToRGB(hexColor int) (int, int, int) {
	r := (hexColor & 0xff0000) >> 16
	g := (hexColor & 0x00ff00) >> 8
	b := (hexColor & 0x0000ff)
	return r, g, b
}

func hueToRGB(p, q, t float64) float64 {
	if t < 0 {
		t += 1
	}
	if t > 1 {
		t -= 1
	}

	if t < (1.0 / 6.0) {
		return p + (q-p)*6*t
	}

	if t < 0.5 {
		return q
	}

	if t < (2.0 / 3.0) {
		return p + (q-p)*(2.0/3.0-t)*6
	}

	return p
}

// converts an RGB color to HSL where H is between 0 and 360, and S, L are
// between 0 and 1.
func rgbToHSL(r, g, b int) (float64, float64, float64) {
	// get RGB as values between 0 and 1
	rf := float64(r) / 255
	gf := float64(g) / 255
	bf := float64(b) / 255

	max := math.Max(rf, math.Max(gf, bf))
	min := math.Min(rf, math.Min(gf, bf))

	var h, s float64
	var l float64 = (max + min) / 2

	if max == min {
		h = 0
		s = 0
	} else {
		d := max - min

		s = d / (max + min)
		if l > 0.5 {
			s = d / (2 - max - min)
		}

		switch max {
		case rf:
			var x float64 = 0
			if gf < bf {
				x = 6
			}

			h = (gf-bf)/d + x
		case gf:
			h = (bf-rf)/d + 2
		case bf:
			h = (rf-gf)/d + 4
		}

		h /= 6
	}

	return h, s, l
}

// converts an HSL color to RGB
func hslToRGB(h, s, l float64) (int, int, int) {
	var rf, gf, bf float64

	if s == 0 {
		// achromatic!
		rf = 1
		gf = 1
		bf = 1
	} else {
		q := l + s - (l * s)
		if l < 0.5 {
			q = l * (1 + s)
		}

		p := (2 * l) - q

		third := 1.0 / 3.0
		rf = hueToRGB(p, q, h+third)
		gf = hueToRGB(p, q, h)
		bf = hueToRGB(p, q, h-third)
	}

	return int(rf * 255), int(gf * 255), int(bf * 255)
}

func color(colorCode int) string {
	return fmt.Sprintf("%s[3%dm", COLOR_ESCAPE, colorCode)
}

func colored(s string, colorCode int) string {
	fg := color(colorCode)
	return fg + s + COLOR_RESET
}

// given a hex color, turns it into a true-color xterm escape sequence using
// semicolons as parameter delimiters, with no background color.
func trueColor(hexColor, specifierCode int) string {
	r, g, b := hexToRGB(hexColor)
	return fmt.Sprintf("%s[%d;2;%d;%d;%dm", COLOR_ESCAPE, specifierCode, r, g, b)
}

// given a string, returns the string in the given color using xterm true-color
// escape codes.
func trueColored(s string, hexColor int) string {
	fg := trueColor(hexColor, COLOR_CODE_FOREGROUND)
	return fg + s + COLOR_RESET
}
