package main

import (
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"github.com/bwmarrin/discordgo"
)

type GoogleResult struct {
	ResultURL   string
	ResultTitle string
}

type Term struct {
	Usage      string
	Definition string
	Example    string
}

/**
Scrapes Google search for the first <resultCount> results that come from the query.
*/
func fetchResults(query string, resultCount int) []GoogleResult {
	results := []GoogleResult{}

	// Request the HTML page.
	query = url.QueryEscape(query)
	res, err := http.Get(fmt.Sprintf("https://www.google.com/search?q=%s&num=100&hl=en", query))
	if err != nil {
		fmt.Println("Error getting the page.")
		fmt.Println(err)
		return results
	}
	defer res.Body.Close()
	if res.StatusCode != 200 {
		fmt.Println("Page did not return 200 status OK")
		return results
	}

	// Load the HTML document
	doc, err := goquery.NewDocumentFromReader(res.Body)
	if err != nil {
		return results
	}

	selection := doc.Find("div.kCrYT")
	selection.Each(func(i int, s *goquery.Selection) {
		if len(results) < resultCount {
			resultTitle := s.Find("h3").Text()
			resultUrl := s.Find("a").AttrOr("href", "nil")
			resultUrl = strings.TrimPrefix(resultUrl, "/url?q=")
			if resultUrl != "nil" && resultTitle != "" {
				result := GoogleResult{
					resultUrl,
					resultTitle,
				}
				results = append(results, result)
			}
		}
	})
	fmt.Printf("%+v\n", results)
	return results
}

/**
Pulls definitions from the Cambridge dictionary and returns it as
an array of Terms.
*/
func fetchDefinitions(query string) []Term {
	// Request the HTML page.
	terms := []Term{}

	fmt.Println("Query: `" + query + "`")
	res, err := http.Get(fmt.Sprintf("https://dictionary.cambridge.org/us/dictionary/english/%s", query))
	if err != nil {
		fmt.Println("Error getting the page.")
		fmt.Println(err)
		return terms
	}
	defer res.Body.Close()
	if res.StatusCode != 200 {
		fmt.Println("Page did not return 200 status OK")
		return terms
	}

	// Load the HTML document
	doc, err := goquery.NewDocumentFromReader(res.Body)
	if err != nil {
		return terms
	}

	// int index
	doc.Find("div.entry-body__el").Each(func(i int, s1 *goquery.Selection) {
		// get usage:
		usage := strings.Split(s1.Find("div.posgram.dpos-g.hdib.lmr-5").First().Text(), " ")[0]
		definition := s1.Find("div.def.ddef_d.db").First().Text()
		example := s1.Find("div.examp").First().Text()

		term := Term{
			usage,
			definition,
			example,
		}
		fmt.Println("As a " + usage + ", " + query + " means: " + strings.TrimSuffix(definition, ": ") + ".\n-- " + example)
		terms = append(terms, term)
	})

	return terms
}

/**
Defines a word using the Cambridge dictionary and sends the definition back to the channel.
*/
func Handle_define(s *discordgo.Session, m *discordgo.MessageCreate, command []string) {
	if len(command) == 1 {
		s.ChannelMessageSend(m.ChannelID, "Usage: `~define <word/phrase>`")
	} else {
		query := url.QueryEscape(strings.Join(command[1:], "-"))
		terms := fetchDefinitions(query)

		if len(terms) == 0 {
			s.ChannelMessageSend(m.ChannelID, ":books: :frowning: Couldn't find a definition for that in here...")
		} else {
			var embed discordgo.MessageEmbed
			embed.URL = fmt.Sprintf("https://dictionary.cambridge.org/us/dictionary/english/%s", query)
			embed.Type = "rich"
			embed.Title = "Definitions for \"" + strings.Join(command[1:], " ") + "\""
			var fields []*discordgo.MessageEmbedField
			for _, term := range terms {
				var field discordgo.MessageEmbedField
				field.Name = term.Usage
				value := term.Definition + "\n"
				if term.Example != "" {
					value += "`" + term.Example + "`"
				}
				field.Value = value
				field.Inline = false
				fields = append(fields, &field)
			}
			embed.Fields = fields
			var footer discordgo.MessageEmbedFooter
			footer.Text = "Fetched from Cambridge Dictionary"
			footer.IconURL = "https://seeklogo.com/images/U/university-of-cambridge-logo-E6ED593FBF-seeklogo.com.png"
			embed.Footer = &footer

			// send response
			s.ChannelMessageSendEmbed(m.ChannelID, &embed)
		}
	}
}

/**
Sends the first five search results for the query input by the user
*/
func Handle_google(s *discordgo.Session, m *discordgo.MessageCreate, command []string) {
	if len(command) == 1 {
		s.ChannelMessageSend(m.ChannelID, "Usage: `~google <word / phrase>`")
		return
	}
	results := fetchResults(strings.Join(command[1:], " "), 5)
	fmt.Println("Here's the results!")

	if len(results) == 0 {
		s.ChannelMessageSend(m.ChannelID, "Unable to fetch Google results. Try again later :frowning:")
	} else {
		// construct embed response
		var embed discordgo.MessageEmbed
		embed.URL = fmt.Sprintf("https://www.google.com/search?q=%s&num=100&hl=en", url.QueryEscape(strings.Join(command[2:], " ")))
		embed.Type = "rich"
		embed.Title = "Search Results for \"" + strings.Join(command[1:], " ") + "\""
		resultString := ""
		for i, result := range results {
			resultString += fmt.Sprintf("%d: [%s](%s)\n", (i + 1), result.ResultTitle, result.ResultURL)
		}
		embed.Description = resultString
		var footer discordgo.MessageEmbedFooter
		footer.Text = "First " + strconv.Itoa(len(results)) + " results from Google Search Engine"
		footer.IconURL = "https://cdn4.iconfinder.com/data/icons/new-google-logo-2015/400/new-google-favicon-512.png"
		embed.Footer = &footer
		// send response
		s.ChannelMessageSendEmbed(m.ChannelID, &embed)
	}
}

func Handle_image(s *discordgo.Session, m *discordgo.MessageCreate, command []string) {

}

func createCommand(title string, description string) *discordgo.MessageEmbedField {
	var command discordgo.MessageEmbedField
	command.Name = title
	command.Value = description
	command.Inline = false
	return &command
}

func Handle_help(s *discordgo.Session, m *discordgo.MessageCreate) {
	// return all current commands and what they do
	var embed discordgo.MessageEmbed
	embed.Type = "rich"
	embed.Title = "How to use ZawackiBot"
	var thumbnail discordgo.MessageEmbedThumbnail
	thumbnail.URL = "https://static.thenounproject.com/png/1248-200.png"
	embed.Thumbnail = &thumbnail
	var commands []*discordgo.MessageEmbedField
	commands = append(commands, createCommand("~uptime", "Reports the bot's current uptime."))
	commands = append(commands, createCommand("~shutdown", "Shuts the bot down cleanly. Note that if the bot is deployed on an automatic service such as Heroku it will automatically restart."))
	commands = append(commands, createCommand("~nick @user <nickname>", "Renames the specified user to the provided nickname."))
	commands = append(commands, createCommand("~kick @user (reason: optional)", "Kicks the specified user from the server."))
	commands = append(commands, createCommand("~ban @user (reason:optional)", "Bans the specified user from the server."))
	commands = append(commands, createCommand("~perk <perk name>", "Returns the description of the specified Dead by Daylight perk."))
	commands = append(commands, createCommand("~shrine", "Returns the current shrine according to the Dead by Daylight Wiki."))
	commands = append(commands, createCommand("~autoshrine <#channel>", "Changes the channel where Tweets about the newest shrine from @DeadbyBHVR are posted."))
	commands = append(commands, createCommand("~define <word/phrase>", "Returns a definition of the word/phrase if it is available."))
	commands = append(commands, createCommand("~google <word/phrase>", "Returns the first five google results returned from the query."))
	commands = append(commands, createCommand("~image <word/phrase>", "Returns the first image from Google Images."))
	commands = append(commands, createCommand("~help", "Returns how to use each of the commands the bot has available."))
	embed.Fields = commands
	var footer discordgo.MessageEmbedFooter
	footer.Text = "Created by Charles Zawacki; Written in Go"
	footer.IconURL = "https://avatars0.githubusercontent.com/u/44577941?s=460&u=4eb7b9ff5410be189eea9863c33916c805dbd2b2&v=4"
	embed.Footer = &footer
	// send response
	s.ChannelMessageSendEmbed(m.ChannelID, &embed)

}
