package main

import (
	"strings"
	"testing"
)

/**
Test anything that uses calculation outside of the discordgo commands.
Those commands don't need to be tested since they are verified to work
at github.com/bwmarrin/discordgo
**/
func TestDBD(t *testing.T) {
	// t.Run("Shrine scrapes correctly", func(t *testing.T) {
	// 	shrine := scrapeShrine()
	// 	perkCount := 4
	// 	if len(shrine.Perks) != perkCount || len(shrine.Prices) != perkCount || len(shrine.Owners) != perkCount {
	// 		t.Logf("Failed to pull the expected %d perks", perkCount)
	// 		t.Fail()
	// 	}
	// 	if shrine.TimeUntilReset == "" {
	// 		t.Logf("Failed to detect time until shrine resets")
	// 		t.Fail()
	// 	}
	// })

	// just using one perk. this will fail if the design scheme for perks
	// the website changes significantly.
	t.Run("Perks scrape correctly", func(t *testing.T) {
		perk := scrapePerk("Lithe")
		if perk.PageURL != "https://deadbydaylight.gamepedia.com/Lithe" {
			t.Logf("Failed to pull from correct URL")
			t.Fail()
		}
		if strings.Contains(perk.IconURL, "Lithe.gif") == false {
			t.Logf("Failed to pull correct icon")
			t.Fail()
		}
		if perk.Name != "Lithe" {
			t.Logf("Failed to pull the correct perk")
			t.Fail()
		}
		if perk.Quote != "\"U mad?\" — Feng Min " {
			t.Logf("Failed to populate quote correctly: '" + perk.Quote + "'")
			t.Fail()
		}
	})
}
