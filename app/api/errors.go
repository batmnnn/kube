package main

import "errors"

var (
	errInvalidWord     = errors.New("invalid word: use one word, letters only, 2–45 characters")
	errNotInDictionary = errors.New("not in the dictionary — that looks like random letters")
)
