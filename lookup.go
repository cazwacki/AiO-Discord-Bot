package main

import (
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"github.com/bwmarrin/discordgo"
	"google.golang.org/api/customsearch/v1"
	"google.golang.org/api/googleapi/transport"
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

type ImageSet struct {
	Query     string
	MessageID string
	Images    []string
	Index     int
}

/**
Scrapes Google search for the first <resultCount> results that come from the query.
*/
func fetchResults(query string, resultCount int) []GoogleResult {
	results := []GoogleResult{}

	// Request the HTML page.
	query = url.QueryEscape(query)
	doc := loadPage(fmt.Sprintf("https://www.google.com/search?q=%s&num=100&hl=en", query))

	if doc == nil {
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
	doc := loadPage(fmt.Sprintf("https://dictionary.cambridge.org/us/dictionary/english/%s", query))

	if doc == nil {
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
Uses Google CustomSearch API to generate and return 10 images.
*/
func fetch_image(query string) ImageSet {
	fmt.Println("Query: '" + query + "'")
	var newset ImageSet
	client := &http.Client{Transport: &transport.APIKey{Key: os.Getenv("GOOGLE_API_KEY")}}

	svc, err := customsearch.New(client)
	if err != nil {
		fmt.Println(err)
		return newset
	}

	resp, err := svc.Cse.List().Cx("007244931007990492385:f42b7zsrt0k").SearchType("image").Q(query).Do()
	if err != nil {
		fmt.Println(err)
		return newset
	}

	for _, result := range resp.Items {
		newset.Images = append(newset.Images, result.Link)
	}

	newset.Query = query
	newset.Index = 0

	return newset
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
		return
	}
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

/**
Creates and populates an ImageSet to be added to the globalImageSet. Sends the image
to the channel with emotes that can be used to scroll between images.
*/
func Handle_image(s *discordgo.Session, m *discordgo.MessageCreate, command []string) {
	if len(command) == 1 {
		s.ChannelMessageSend(m.ChannelID, "Usage: `~image <word / phrase>`")
		return
	}
	result := fetch_image(strings.Join(command[1:], " "))
	if len(result.Images) == 0 {
		s.ChannelMessageSend(m.ChannelID, ":frame_photo: :frowning: Couldn't find that for you.")
		return
	}
	// craft response and send
	var embed discordgo.MessageEmbed
	embed.Type = "rich"
	embed.Title = "Image Results for \"" + strings.Join(command[1:], " ") + "\""
	var image discordgo.MessageEmbedImage
	image.URL = result.Images[0]
	embed.Image = &image
	var footer discordgo.MessageEmbedFooter
	footer.Text = fmt.Sprintf("Image 1 of %d", len(result.Images))
	footer.IconURL = "https://cdn4.iconfinder.com/data/icons/new-google-logo-2015/400/new-google-favicon-512.png"
	embed.Footer = &footer
	message, _ := s.ChannelMessageSendEmbed(m.ChannelID, &embed)

	result.MessageID = message.ID
	appendToGlobalImageSet(result)

	s.MessageReactionAdd(m.ChannelID, result.MessageID, "⬅️")
	s.MessageReactionAdd(m.ChannelID, result.MessageID, "➡️")
	s.MessageReactionAdd(m.ChannelID, result.MessageID, "⏹️")

}
