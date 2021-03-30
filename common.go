package main

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"github.com/bwmarrin/discordgo"
)

/**
Fetches a response from the requested URL and returns
it in the form of a goquery Document, which can be
searched more easily.
*/
func loadPage(url string) *goquery.Document {
	res, err := http.Get(url)
	if err != nil {
		fmt.Println("Error getting the page.")
		fmt.Println(err)
		return nil
	}
	defer res.Body.Close()
	if res.StatusCode != 200 {
		fmt.Println("Page did not return 200 status OK")
		return nil
	}

	// Load the HTML document
	doc, err := goquery.NewDocumentFromReader(res.Body)
	if err != nil {
		fmt.Println("Error converting response into goquery Document")
		return nil
	}

	return doc
}

/**
Returns a field to be added to a Discord embed message.
Used to prevent bloating in the methods where it is used.
*/
func createField(title string, description string, inline bool) *discordgo.MessageEmbedField {
	var command discordgo.MessageEmbedField
	command.Name = title
	command.Value = description
	command.Inline = inline
	return &command
}

/**
Strips the characters surrounding a user ID. Heavily used,
so it warrants a method.
*/
func stripUserID(raw string) string {
	// strip <@ and > surrounding the user ID
	raw = strings.TrimSuffix(raw, ">")
	raw = strings.TrimPrefix(raw, "<@")

	// remove the ! if the user has a nickname
	raw = strings.TrimPrefix(raw, "!")
	return raw
}
