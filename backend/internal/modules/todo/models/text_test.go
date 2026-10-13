package models

import "testing"

func TestNormalizeTitle(t *testing.T) {
	title, message := NormalizeTitle("  Buy milk  ")
	if message != "" || title != "Buy milk" {
		t.Fatalf("title %q message %q", title, message)
	}
	if _, message := NormalizeTitle("   "); message != "title is required" {
		t.Fatal(message)
	}
	if _, message := NormalizeTitle(string(make([]byte, MaxTitleLength+1))); message == "" {
		t.Fatal("expected length error")
	}
}

func TestNormalizeDescriptionAndQuery(t *testing.T) {
	if _, message := NormalizeDescription(string(make([]rune, MaxDescriptionLength+1))); message == "" {
		t.Fatal("expected description length error")
	}
	query, message := NormalizeQuery("  milk ")
	if message != "" || query != "milk" {
		t.Fatalf("query %q message %q", query, message)
	}
}
