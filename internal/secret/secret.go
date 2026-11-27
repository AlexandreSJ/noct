// Package secret is how noct hides a secret value wherever it would show one: a dot for each
// character while it is typed, and one mask, which keeps its length unknown, once it is a value.
package secret

import (
	"strings"
	"unicode/utf8"
)

// Dot stands for each character of a secret while it is typed.
const Dot = "•"

// Mask stands for a secret value, whatever its length.
const Mask = "••••"

// Typed is text as a field shows a secret being typed: a dot a character.
func Typed(text string) string {
	return strings.Repeat(Dot, utf8.RuneCountInString(text))
}

// Value is a secret value as noct shows it: the mask, or "" for no value.
func Value(value string) string {
	if value == "" {
		return ""
	}
	return Mask
}

// Show is value as noct shows it: masked when hidden says it is a secret.
func Show(value string, hidden bool) string {
	if hidden {
		return Value(value)
	}
	return value
}

// Hide is text with each of the secret values in it masked.
func Hide(text string, values []string) string {
	for _, value := range values {
		if value != "" {
			text = strings.ReplaceAll(text, value, Mask)
		}
	}
	return text
}
