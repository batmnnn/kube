package main

import (
	"math"
	"regexp"
	"strings"
	"unicode"
)

const (
	maxWordLen    = 45
	maxLenPoints  = 500.0
	maxRarePoints = 500.0
)

var wordPattern = regexp.MustCompile(`^[a-zA-Z]+$`)

type scoreBreakdown struct {
	Word             string `json:"word"`
	Length           int    `json:"length"`
	Score            int    `json:"score"`
	LengthPoints     int    `json:"length_points"`
	UniquenessPoints int    `json:"uniqueness_points"`
	CorpusFrequency  int64  `json:"corpus_frequency,omitempty"`
	RarityTier       string `json:"rarity_tier"`
}

func normalizeWord(raw string) (string, error) {
	word := strings.TrimSpace(raw)
	if word == "" {
		return "", errInvalidWord
	}
	if strings.Contains(word, " ") {
		return "", errInvalidWord
	}
	word = strings.ToLower(word)
	if len(word) < 2 || len(word) > maxWordLen {
		return "", errInvalidWord
	}
	if !wordPattern.MatchString(word) {
		return "", errInvalidWord
	}
	if !isDictionaryWord(word) {
		return "", errNotInDictionary
	}
	return word, nil
}

func calculateScore(word string) scoreBreakdown {
	length := len(word)

	lengthRatio := float64(length) / float64(maxWordLen)
	if lengthRatio > 1 {
		lengthRatio = 1
	}
	lengthPoints := lengthRatio * maxLenPoints

	// Rarity from real-world English usage (Google Books / OpenSubtitles corpus).
	uniquenessPoints, corpusFreq, tier := corpusRarityPoints(word)

	total := int(math.Round(lengthPoints + uniquenessPoints))
	if total < 1 {
		total = 1
	}
	if total > 1000 {
		total = 1000
	}

	return scoreBreakdown{
		Word:             word,
		Length:           length,
		Score:            total,
		LengthPoints:     int(math.Round(lengthPoints)),
		UniquenessPoints: int(math.Round(uniquenessPoints)),
		CorpusFrequency:  corpusFreq,
		RarityTier:       tier,
	}
}

func displayWord(word string) string {
	if word == "" {
		return word
	}
	runes := []rune(word)
	runes[0] = unicode.ToUpper(runes[0])
	return string(runes)
}
