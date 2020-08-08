package main

import (
	"testing"
)

/**
Test anything that uses calculation outside of the discordgo commands.
Those commands don't need to be tested since they are verified to work
at github.com/bwmarrin/discordgo
**/
func TestLookups(t *testing.T) {
	t.Run("~image scrapes 10 images correctly", func(t *testing.T) {
		imageSet := fetchImage("gecko")
		if imageSet.Query != "gecko" {
			t.Logf("Failed to populate query correctly: %s", imageSet.Query)
			t.Fail()
		}
		if len(imageSet.Images) != 10 {
			t.Logf("Failed to pull 10 images for the query: Got %d", len(imageSet.Images))
			t.Fail()
		}
	})

	t.Run("~google returns first five results and they are populated", func(t *testing.T) {
		results := fetchResults("blacksburg restaurants", 5)
		if len(results) != 5 {
			t.Logf("Failed to find 5 results for the common query, found %d", len(results))
			t.Fail()
		}
		for _, result := range results {
			if result.ResultURL == "" || result.ResultTitle == "" {
				t.Logf("Failed to populate results correctly, %+v", result)
				t.Fail()
			}
		}
	})

	t.Run("~define returns a valid definition", func(t *testing.T) {
		terms := fetchDefinitions("test")
		if len(terms) == 0 {
			t.Logf("Failed to find usages for word we know exists, found %d results", len(terms))
			t.Fail()
		}
		for _, term := range terms {
			if term.Usage == "" || term.Definition == "" {
				t.Logf("Failed to populate usage and definition correctly %+v", term)
				t.Fail()
			}
		}
	})
}
