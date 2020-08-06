package main

import (
	"fmt"
	"net/http"

	"github.com/PuerkitoBio/goquery"
	"github.com/bwmarrin/discordgo"
)

func Handle_define(s *discordgo.Session, m *discordgo.MessageCreate, command []string) {
	if len(command) == 1 {
		s.ChannelMessageSend(m.ChannelID, "Usage: `~define <word/phrase>`")
	} else {
		// Request the HTML page.
		res, err := http.Get("")
		if err != nil {
			fmt.Println("Error getting the page.")
			fmt.Println(err)
			return
		}
		defer res.Body.Close()
		if res.StatusCode != 200 {
			fmt.Println("Page did not return 200 status OK")
			return
		}

		// Load the HTML document
		doc, err := goquery.NewDocumentFromReader(res.Body)
		if err != nil {
			doc.Find("")
			return
		}
	}
}

func Handle_google(s *discordgo.Session, m *discordgo.MessageCreate, command []string) {

}

func Handle_image(s *discordgo.Session, m *discordgo.MessageCreate, command []string) {

}

func Handle_help(s *discordgo.Session, m *discordgo.MessageCreate) {

}
