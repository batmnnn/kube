package main

import (
	"bufio"
	"embed"
	"log/slog"
	"strings"
)

//go:embed data/words_alpha.txt
var dictionaryFile embed.FS

var dictionary map[string]struct{}

func init() {
	dictionary = loadDictionary()
	slog.Info("dictionary loaded", "words", len(dictionary))
}

func loadDictionary() map[string]struct{} {
	f, err := dictionaryFile.Open("data/words_alpha.txt")
	if err != nil {
		slog.Error("dictionary load failed", "error", err)
		return map[string]struct{}{}
	}
	defer f.Close()

	words := make(map[string]struct{}, 380_000)
	scanner := bufio.NewScanner(f)
	buf := make([]byte, 64*1024)
	scanner.Buffer(buf, 1024*1024)
	for scanner.Scan() {
		w := strings.TrimSpace(scanner.Text())
		if w != "" {
			words[w] = struct{}{}
		}
	}
	if err := scanner.Err(); err != nil {
		slog.Error("dictionary read failed", "error", err)
	}
	return words
}

func isDictionaryWord(word string) bool {
	_, ok := dictionary[strings.ToLower(word)]
	return ok
}
