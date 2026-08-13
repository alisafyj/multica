package designcore

import (
	"math"
	"unicode"
)

type TypographyMetrics struct {
	FontSize      float64
	LetterSpacing float64
}

func MeasureTextWidth(text string, metrics TypographyMetrics) float64 {
	if !isNonNegativeFinite(metrics.FontSize) || !isFinite(metrics.LetterSpacing) {
		return 0
	}

	width := 0.0
	runeCount := 0
	for _, r := range text {
		width = addWidth(width, runeWidthFactor(r)*metrics.FontSize)
		runeCount++
	}
	if runeCount > 1 {
		width = addWidth(width, float64(runeCount-1)*metrics.LetterSpacing)
	}
	if width < 0 {
		return 0
	}
	return width
}

func runeWidthFactor(r rune) float64 {
	switch {
	case unicode.IsMark(r):
		return 0
	case isFullWidthRune(r):
		return 1.0
	case r >= 'A' && r <= 'Z':
		return 0.68
	case (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9'):
		return 0.56
	case unicode.IsSpace(r):
		return 0.33
	case unicode.IsPunct(r):
		return 0.4
	default:
		return 1.0
	}
}

func isFullWidthRune(r rune) bool {
	return unicode.Is(unicode.Han, r) ||
		unicode.Is(unicode.Hangul, r) ||
		unicode.Is(unicode.Hiragana, r) ||
		unicode.Is(unicode.Katakana, r) ||
		unicode.Is(unicode.Bopomofo, r) ||
		(r >= 0x3000 && r <= 0x303F) ||
		(r >= 0xFF01 && r <= 0xFF60) ||
		(r >= 0xFFE0 && r <= 0xFFE6)
}

func isNonNegativeFinite(value float64) bool {
	return value >= 0 && isFinite(value)
}

func isFinite(value float64) bool {
	return !math.IsNaN(value) && !math.IsInf(value, 0)
}

func addWidth(width, addition float64) float64 {
	if math.MaxFloat64-width < addition {
		return math.MaxFloat64
	}
	return width + addition
}
