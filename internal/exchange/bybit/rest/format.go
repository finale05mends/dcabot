package rest

import (
	"math"
	"strconv"
	"strings"
)

func formatWithStep(value, step float64) string {
	if step <= 0 {
		return strconv.FormatFloat(value, 'f', -1, 64)
	}

	decimals := stepDecimals(step)
	quantized := math.Floor((value/step)+1e-9) * step

	return strconv.FormatFloat(quantized, 'f', decimals, 64)
}

func stepDecimals(step float64) int {
	text := strconv.FormatFloat(step, 'f', -1, 64)

	if strings.Contains(text, "e") || strings.Contains(text, "E") {
		text = strconv.FormatFloat(step, 'f', 18, 64)
	}

	if dot := strings.IndexByte(text, '.'); dot >= 0 {
		return len(strings.TrimRight(text[dot+1:], "0"))
	}

	return 0
}
