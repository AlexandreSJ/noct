package kittygfx

import "image/color"

// placeholderRune marks a cell as part of an image; diacritics say which row and column.
const placeholderRune = '\U0010EEEE'

// MaxPlacementCells is the largest row or column a placeholder can express.
const MaxPlacementCells = len(rowColumnDiacritics)

// PlaceholderText is the content of the cell at row, col of an image. IDs above 24 bits add a
// third diacritic with the most significant byte.
func PlaceholderText(imageID uint32, row, col int) string {
	text := []rune{placeholderRune, rowColumnDiacritics[row], rowColumnDiacritics[col]}
	if high := imageID >> 24; high != 0 {
		text = append(text, rowColumnDiacritics[high])
	}
	return string(text)
}

// PlaceholderColor is the foreground color of a placeholder cell: the image id's low 24 bits.
func PlaceholderColor(imageID uint32) color.Color {
	return color.RGBA{R: uint8(imageID >> 16), G: uint8(imageID >> 8), B: uint8(imageID), A: 0xff}
}
