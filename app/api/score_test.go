package main

import "testing"

func TestCalculateScore(t *testing.T) {
	longRare := calculateScore("antidisestablishmentarianism")
	if longRare.Score < 700 || longRare.Score > 1000 {
		t.Fatalf("expected high long obscure word score, got %d", longRare.Score)
	}
	if longRare.RarityTier != "obscure" {
		t.Fatalf("expected obscure tier, got %q", longRare.RarityTier)
	}

	hello := calculateScore("hello")
	disaster := calculateScore("disaster")
	articulate := calculateScore("articulate")

	if hello.UniquenessPoints >= disaster.UniquenessPoints {
		t.Fatalf("hello (%d) should score lower rarity than disaster (%d)", hello.UniquenessPoints, disaster.UniquenessPoints)
	}
	if disaster.UniquenessPoints >= articulate.UniquenessPoints {
		t.Fatalf("disaster (%d) should score lower rarity than articulate (%d)", disaster.UniquenessPoints, articulate.UniquenessPoints)
	}

	if hello.RarityTier != "common" {
		t.Fatalf("hello should be common, got %q", hello.RarityTier)
	}
	if articulate.RarityTier != "rare" {
		t.Fatalf("articulate should be rare, got %q", articulate.RarityTier)
	}

	min := calculateScore("go")
	if min.Score < 1 || min.Score > 1000 {
		t.Fatalf("score out of range: %d", min.Score)
	}
}

func TestNormalizeWord(t *testing.T) {
	word, err := normalizeWord("  Hello  ")
	if err != nil || word != "hello" {
		t.Fatalf("expected hello, got %q err=%v", word, err)
	}

	if _, err := normalizeWord("two words"); err == nil {
		t.Fatal("expected error for spaces")
	}

	if _, err := normalizeWord("iashihaoisdcihs"); err != errNotInDictionary {
		t.Fatalf("expected dictionary error, got %v", err)
	}

	if _, err := normalizeWord("hello"); err != nil {
		t.Fatalf("hello should be valid, got %v", err)
	}
}

func TestCorpusRarityOrdering(t *testing.T) {
	_, _, tierDisaster := corpusRarityPoints("disaster")
	_, _, tierArticulate := corpusRarityPoints("articulate")
	if tierDisaster == tierArticulate {
		t.Fatal("expected different rarity tiers for disaster vs articulate")
	}
}
