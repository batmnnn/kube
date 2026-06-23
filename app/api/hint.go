package main

import (
	"math/rand"
	"net/http"
)

var hintCandidates []string

type hintResponse struct {
	Word   string `json:"word"`
	Length int    `json:"length"`
	Tier   string `json:"rarity_tier"`
	Tip    string `json:"tip"`
}

func buildHintCandidates() {
	for word, rank := range wordRank {
		if rank >= 12_000 && len(word) >= 8 && isDictionaryWord(word) {
			hintCandidates = append(hintCandidates, word)
		}
	}
}

func randomHint() hintResponse {
	if len(hintCandidates) == 0 {
		return hintResponse{Tip: "No hints available yet"}
	}
	word := hintCandidates[rand.Intn(len(hintCandidates))]
	_, _, tier := corpusRarityPoints(word)
	return hintResponse{
		Word:   displayWord(word),
		Length: len(word),
		Tier:   tier,
		Tip:    "Rare in everyday English — high uniqueness points if you type it in time!",
	}
}

func getHint() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, randomHint())
	}
}
