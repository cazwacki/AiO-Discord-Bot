package main

import (
	"fmt"
	"net/http"
	"runtime"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"github.com/bwmarrin/discordgo"
)

/**
Prints an info log to the console if debug mode is on.
*/
func logInfo(message string) {
	if debug {
		pc, _, _, _ := runtime.Caller(1)
		funcCalledIn := runtime.FuncForPC(pc).Name()
		fmt.Printf("[INFO] %s: %s\n", funcCalledIn, message)
	}
}

/**
Prints an info log to the console if debug mode is on.
*/
func logWarning(message string) {
	if debug {
		pc, _, _, _ := runtime.Caller(1)
		funcCalledIn := runtime.FuncForPC(pc).Name()
		fmt.Printf("[\033[33mWARN\033[0m] %s: %s\n", funcCalledIn, message)
	}
}

/**
Prints an info log to the console if debug mode is on.
*/
func logError(message string) {
	if debug {
		pc, _, _, _ := runtime.Caller(1)
		funcCalledIn := runtime.FuncForPC(pc).Name()
		fmt.Printf("[\033[31mERR!\033[0m] %s: %s\n", funcCalledIn, message)
	}
}

/**
Prints a success log to the console if debug mode is on.
*/
func logSuccess(message string) {
	if debug {
		pc, _, _, _ := runtime.Caller(1)
		funcCalledIn := runtime.FuncForPC(pc).Name()
		fmt.Printf("[\033[32m OK \033[0m] %s: %s\n", funcCalledIn, message)
	}
}

/**
Fetches a response from the requested URL and returns
it in the form of a goquery Document, which can be
searched more easily.
*/
func loadPage(url string) *goquery.Document {
	res, err := http.Get(url)
	if err != nil {
		logError("Error on GET request." + err.Error())
		return nil
	}
	defer res.Body.Close()
	if res.StatusCode != 200 {
		logError("Page did not return 200 status OK")
		return nil
	}

	// Load the HTML document
	doc, err := goquery.NewDocumentFromReader(res.Body)
	if err != nil {
		logError("Error converting response into goquery document. " + err.Error())
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
