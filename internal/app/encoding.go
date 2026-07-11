package app

import (
	"bytes"
	"unicode/utf8"
)

type EncodingInfo struct {
	Encoding       string `json:"encoding"`
	HasBom         bool   `json:"hasBom"`
	Confidence     string `json:"confidence"`
	IsIncompatible bool   `json:"isIncompatible"`
}

func detectEncoding(data []byte) EncodingInfo {
	if len(data) >= 3 && bytes.HasPrefix(data, []byte{0xEF, 0xBB, 0xBF}) {
		return EncodingInfo{Encoding: "utf-8", HasBom: true, Confidence: "high"}
	}
	if len(data) >= 2 && bytes.HasPrefix(data, []byte{0xFF, 0xFE}) {
		return EncodingInfo{Encoding: "utf-16le", HasBom: true, Confidence: "high"}
	}
	if len(data) >= 2 && bytes.HasPrefix(data, []byte{0xFE, 0xFF}) {
		return EncodingInfo{Encoding: "utf-16be", HasBom: true, Confidence: "high"}
	}
	if enc, ok := detectUtf16ByNullPattern(data); ok {
		return EncodingInfo{Encoding: enc, HasBom: false, Confidence: "medium"}
	}
	if utf8.Valid(data) {
		return EncodingInfo{Encoding: "utf-8", HasBom: false, Confidence: "high"}
	}
	return EncodingInfo{Encoding: "unknown", IsIncompatible: true, Confidence: "low"}
}

func detectUtf16ByNullPattern(data []byte) (string, bool) {
	sample := data
	if len(sample) > 4096 {
		sample = sample[:4096]
	}
	if len(sample) < 4 {
		return "", false
	}
	evenZero, oddZero := 0, 0
	for i := 0; i+1 < len(sample); i += 2 {
		if sample[i] == 0 {
			evenZero++
		}
		if sample[i+1] == 0 {
			oddZero++
		}
	}
	total := len(sample) / 2
	if total == 0 {
		return "", false
	}
	evenRatio := float64(evenZero) / float64(total)
	oddRatio := float64(oddZero) / float64(total)
	if oddRatio >= 0.6 && evenRatio <= 0.2 {
		return "utf-16le", true
	}
	if evenRatio >= 0.6 && oddRatio <= 0.2 {
		return "utf-16be", true
	}
	return "", false
}
