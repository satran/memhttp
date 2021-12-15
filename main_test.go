package main

import "testing"

func TestWalk(t *testing.T) {
	files, err := walk(".", ".git")
	if err != nil {
		t.Fatal(err)
	}
	for _, exp := range []string{
		"go.mod",
		"main.go",
		"main_test.go",
		"readme.md",
	} {
		found := false
		for name := range files {
			if name == exp {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected: %s", exp)
		}
	}
}
