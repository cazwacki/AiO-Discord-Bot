package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
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

// JSON Structs for Wikipedia
type Article struct {
	Title   *string       `json:"displaytitle,omitempty"`
	Preview *Thumbnail    `json:"thumbnail,omitempty"`
	URLs    *Content_URLs `json:"content_urls,omitempty"`
	Extract string        `json:"extract,omitempty"`
}

type Thumbnail struct {
	Source string `json:"source"`
}

type Content_URLs struct {
	URLs URLSet `json:"desktop"`
}

type URLSet struct {
	Page string `json:"page"`
}

// JSON Structs for Lingua Bot
type DictResults struct {
	Entries []Entry `json:"entries"`
}

type Entry struct {
	Term        string   `json:"entry"`
	Definitions []Lexeme `json:"lexemes,omitempty"`
	SourceURLs  []string `json:"sourceUrls,omitempty"`
}

type Lexeme struct {
	PartOfSpeech string  `json:"partOfSpeech"`
	Senses       []Sense `json:"senses,omitempty"`
}

type Sense struct {
	Definition string   `json:"definition"`
	Labels     []string `json:"labels,omitempty"`
}

// GoogleResult : holds a result's title and link
type GoogleResult struct {
	ResultURL   string
	ResultTitle string
}

// ImageSet : holds 10 images and the associated message of an image query
type ImageSet struct {
	Query     string
	Message   *discordgo.Message
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
			resultUrl = strings.Split(strings.TrimPrefix(resultUrl, "/url?q="), "&")[0]
			// some URL decoding
			resultUrl = strings.ReplaceAll(resultUrl, "%3F", "?")
			resultUrl = strings.ReplaceAll(resultUrl, "%3D", "=")
			resultUrl = strings.ReplaceAll(resultUrl, "%2520", "%20")
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
Pulls definitions from the Lingua Bot API and returns it as
an array of Entries.
*/
func fetchDefinitions(query string) DictResults {
	var definitions DictResults

	// fetch response from lingua robot API
	url := "https://lingua-robot.p.rapidapi.com/language/v1/entries/en/" + query

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return definitions
	}

	req.Header.Add("x-rapidapi-key", os.Getenv("LINGUA_API_KEY"))
	req.Header.Add("x-rapidapi-host", "lingua-robot.p.rapidapi.com")

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return definitions
	}

	defer res.Body.Close()
	body, err := ioutil.ReadAll(res.Body)

	if err != nil {
		return definitions
	}

	// load json response into definitions struct
	json.Unmarshal(body, &definitions)

	return definitions
}

/**
Uses Google CustomSearch API to generate and return 10 images.
*/
func fetchImage(query string) ImageSet {
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

func fetchArticle(query string) Article {
	var article Article

	// fetch response from lingua robot API
	url := "https://en.wikipedia.org/api/rest_v1/page/summary/" + query

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return article
	}

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return article
	}

	defer res.Body.Close()
	body, err := ioutil.ReadAll(res.Body)

	if err != nil {
		return article
	}

	// load json response into definitions struct
	json.Unmarshal(body, &article)

	return article
}

/**
Defines a word using the Cambridge dictionary and sends the definition back to the channel.
*/
func handleDefine(s *discordgo.Session, m *discordgo.MessageCreate, command []string) {
	// was the command invoked correctly?
	if len(command) == 1 {
		s.ChannelMessageSend(m.ChannelID, "Usage: `~define <word/phrase>`")
		return
	}

	query := url.QueryEscape(strings.Join(command[1:], "-"))
	terms := fetchDefinitions(query)

	// did the API return any definition?
	if len(terms.Entries) == 0 {
		s.ChannelMessageSend(m.ChannelID, ":books: :frowning: Couldn't find a definition for that in here...")
		return
	}

	// construct embed response
	var embed discordgo.MessageEmbed
	embed.Type = "rich"
	embed.Title = "Definitions for \"" + strings.Join(command[1:], " ") + "\""
	var fields []*discordgo.MessageEmbedField
	for _, entry := range terms.Entries {
		for _, definition := range entry.Definitions {
			var field discordgo.MessageEmbedField
			field.Name = definition.PartOfSpeech + " "
			field.Value = ""
			for index, sense := range definition.Senses {
				labels := ""
				if len(sense.Labels) != 0 {
					labels += "(" + strings.Join(sense.Labels, ", ") + ")"
				}
				field.Value += fmt.Sprintf("`%d. %s`\n %s\n\n", index+1, labels, sense.Definition)
			}
			field.Inline = false
			fields = append(fields, &field)
		}
	}
	embed.Fields = fields
	var footer discordgo.MessageEmbedFooter
	footer.Text = "Fetched from Wiktionary"
	footer.IconURL = "https://upload.wikimedia.org/wikipedia/commons/thumb/0/05/WiktionaryEn_-_DP_Derivative.svg/1200px-WiktionaryEn_-_DP_Derivative.svg.png"
	embed.Footer = &footer

	// send response
	s.ChannelMessageSendEmbed(m.ChannelID, &embed)

}

/**
Sends the first five search results for the query input by the user
*/
func handleGoogle(s *discordgo.Session, m *discordgo.MessageCreate, command []string) {
	// was the command invoked correctly?
	if len(command) == 1 {
		s.ChannelMessageSend(m.ChannelID, "Usage: `~google <word / phrase>`")
		return
	}
	results := fetchResults(strings.Join(command[1:], " "), 5)
	fmt.Println("Here's the results!")

	// did any results come in?
	if len(results) == 0 {
		s.ChannelMessageSend(m.ChannelID, "Unable to fetch Google results. Try again later :frowning:")
		return
	}

	// construct embed response
	var embed discordgo.MessageEmbed
	embed.URL = fmt.Sprintf("https://www.google.com/search?q=%s&num=100&hl=en", url.QueryEscape(strings.Join(command[1:], " ")))
	embed.Type = "rich"
	embed.Title = "Search Results for \"" + strings.Join(command[1:], " ") + "\""
	resultString := ""
	for i, result := range results {
		fmt.Printf("result.ResultURL = %s\n", result.ResultURL)
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
func handleImage(s *discordgo.Session, m *discordgo.MessageCreate, command []string) {
	// did the user format the command correctly?
	if len(command) == 1 {
		s.ChannelMessageSend(m.ChannelID, "Usage: `~image <word / phrase>`")
		return
	}
	result := fetchImage(strings.Join(command[1:], " "))

	// did the search engine return anything?
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

	result.Message = message
	go appendToGlobalImageSet(s, result)

	s.MessageReactionAdd(m.ChannelID, result.Message.ID, "⬅️")
	s.MessageReactionAdd(m.ChannelID, result.Message.ID, "➡️")
	s.MessageReactionAdd(m.ChannelID, result.Message.ID, "⏹️")

}

func handleWiki(s *discordgo.Session, m *discordgo.MessageCreate, command []string) {
	if len(command) == 1 {
		s.ChannelMessageSend(m.ChannelID, "Usage: `~wiki <word / phrase>`")
		return
	}
	query := strings.Join(command[1:], "_")
	page := fetchArticle(query)

	if page.URLs == nil {
		s.ChannelMessageSend(m.ChannelID, ":frowning: Couldn't find an article for that. Sorry!")
		return
	}

	var embed discordgo.MessageEmbed
	embed.Type = "rich"
	embed.Title = *page.Title
	embed.Description = page.Extract
	embed.URL = page.URLs.URLs.Page
	var image discordgo.MessageEmbedImage
	if page.Preview != nil {
		image.URL = page.Preview.Source
	}
	embed.Image = &image
	var footer discordgo.MessageEmbedFooter
	footer.Text = "Pulled from Wikipedia"
	footer.IconURL = "https://upload.wikimedia.org/wikipedia/commons/thumb/b/b3/Wikipedia-logo-v2-en.svg/1200px-Wikipedia-logo-v2-en.svg.png"
	embed.Footer = &footer
	s.ChannelMessageSendEmbed(m.ChannelID, &embed)
}
