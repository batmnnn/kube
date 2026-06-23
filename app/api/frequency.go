package main

import (
	"bufio"
	"embed"
	"log/slog"
	"math"
	"strconv"
	"strings"
)

//go:embed data/en_50k.txt
var frequencyFile embed.FS

var (
	wordFrequency map[string]int64
	wordRank      map[string]int
	maxCorpusFreq float64
)

func init() {
	loadCorpusFrequencies()
	slog.Info("corpus frequencies loaded", "words", len(wordFrequency))
}

func loadCorpusFrequencies() {
	f, err := frequencyFile.Open("data/en_50k.txt")
	if err != nil {
		slog.Error("corpus frequency load failed", "error", err)
		wordFrequency = map[string]int64{}
		wordRank = map[string]int{}
		return
	}
	defer f.Close()

	wordFrequency = make(map[string]int64, 50_000)
	wordRank = make(map[string]int, 50_000)

	scanner := bufio.NewScanner(f)
	buf := make([]byte, 64*1024)
	scanner.Buffer(buf, 1024*1024)
	rank := 0
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		parts := strings.Fields(line)
		if len(parts) != 2 {
			continue
		}
		word := strings.ToLower(parts[0])
		freq, err := strconv.ParseInt(parts[1], 10, 64)
		if err != nil {
			continue
		}
		rank++
		wordFrequency[word] = freq
		wordRank[word] = rank
		if float64(freq) > maxCorpusFreq {
			maxCorpusFreq = float64(freq)
		}
	}
	if err := scanner.Err(); err != nil {
		slog.Error("corpus frequency read failed", "error", err)
	}
	buildHintCandidates()
}

func corpusRarityPoints(word string) (points float64, frequency int64, tier string) {
	freq, ok := wordFrequency[word]
	if !ok {
		return maxRarePoints, 0, "obscure"
	}

	// Log scale: "the" ≈ 0 pts, mid words ≈ mid pts, tail ≈ high pts.
	logFreq := math.Log(float64(freq) + 1)
	logMax := math.Log(maxCorpusFreq + 1)
	ratio := 1.0 - (logFreq / logMax)
	points = ratio * maxRarePoints

	return points, freq, rarityTier(wordRank[word])
}

func rarityTier(rank int) string {
	if rank == 0 {
		return "obscure"
	}
	switch {
	case rank <= 500:
		return "common"
	case rank <= 5_000:
		return "everyday"
	case rank <= 15_000:
		return "uncommon"
	default:
		return "rare"
	}
}
