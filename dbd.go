package main

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"strings"
	"unicode"

	"github.com/ChimeraCoder/anaconda"
	"github.com/PuerkitoBio/goquery"
	"github.com/bwmarrin/discordgo"
)

type Perk struct {
	Name         string
	IconURL      string
	Description  string
	Quote        string
	PageURL      string
	HTTPResponse int
}

type Shrine struct {
	Prices         []string
	Perks          []string
	Owners         []string
	TimeUntilReset string
	HTTPResponse   int
}

/**
Helper function for Handle_perk and should be used before scrape_perk.
Formats the perk provided in the command such that it can be used as
the end of the URL query to https://deadbydaylight.gamepedia.com/.
*/
func format_perk(command []string) string {
	specialWords := " in the of for from "
	for i := 1; i < len(command); i++ {
		if strings.Contains(specialWords, command[i]) {
			command[i] = strings.ToLower(command[i])
		} else {
			tmp := []rune(command[i])
			tmp[0] = unicode.ToUpper(tmp[0])
			command[i] = string(tmp)
		}
	}
	perk := strings.Join(command[1:], "_")
	perk = strings.Replace(perk, "_And", "_&", 1)
	perk = url.QueryEscape(perk)
	return perk
}

/**
Helper function for Handle_perk. Scrapes HTML from the respective
page on https://deadbydaylight.gamepedia.com/ and returns the
desired information in the Perk struct created above.
*/
func scrape_perk(perk string) Perk {
	var resultingPerk Perk

	resultingPerk.PageURL = "https://deadbydaylight.gamepedia.com/" + perk

	/********************************
	   GET THE HTML DOCUMENT FIRST
	********************************/
	// Request the HTML page.
	res, err := http.Get(resultingPerk.PageURL)
	if err != nil {
		fmt.Println("Error getting the page.")
		fmt.Println(err)
		resultingPerk.HTTPResponse = 404
		return resultingPerk
	}
	defer res.Body.Close()
	resultingPerk.HTTPResponse = res.StatusCode
	if resultingPerk.HTTPResponse != 200 {
		fmt.Println("Page did not return 200 status OK")
		// try the hex page
		if strings.HasPrefix(perk, "Hex:_") {
			return resultingPerk
		}
		return scrape_perk("Hex:_" + perk)
	}

	// Load the HTML document
	doc, err := goquery.NewDocumentFromReader(res.Body)
	if err != nil {
		return resultingPerk
	}

	/********************************
	   GET THE PERK INFO FROM THE
	         DOCUMENT BODY
	********************************/

	/** Get Perk Name **/
	docName := doc.Find(".firstHeading").First()
	resultingPerk.Name = docName.Text()

	/** Get Description **/
	docDesc := doc.Find(".wikitable").First().Find("td").Last()

	docDescText := strings.ReplaceAll(docDesc.Text(), "\n", " ")
	// remove impurities
	description := strings.ReplaceAll(docDescText, " .", ".")
	description = strings.ReplaceAll(description, "  ", " ")
	description = strings.ReplaceAll(description, " %", "%")

	separatedDescription := strings.Split(description, " \"")

	resultingPerk.Description = separatedDescription[0]
	if len(separatedDescription) > 1 {
		resultingPerk.Quote = "\"" + separatedDescription[1]
	} else {
		resultingPerk.Quote = ""
	}

	/** Get Perk GIF **/
	doc.Find("img").Each(func(i int, s *goquery.Selection) {
		currentSrc := s.AttrOr("src", "nil")
		if strings.Contains(currentSrc, ".gif") {
			resultingPerk.IconURL = currentSrc
		}
	})

	return resultingPerk
}

/**
Helper function for Handle_perk. Scrapes HTML from the shrine
page on https://deadbydaylight.gamepedia.com/ and returns the
desired information in the Shrine struct created above.
*/
func scrape_shrine() Shrine {
	var resultingShrine Shrine
	// Request the HTML page.
	res, err := http.Get("https://deadbydaylight.gamepedia.com/Shrine_of_Secrets")
	if err != nil {
		fmt.Println("Error getting the page.")
		fmt.Println(err)
		return resultingShrine
	}
	defer res.Body.Close()
	resultingShrine.HTTPResponse = res.StatusCode
	if resultingShrine.HTTPResponse != 200 {
		fmt.Println("Page did not return 200 status OK")
		return resultingShrine
	}

	// Load the HTML document
	doc, err := goquery.NewDocumentFromReader(res.Body)
	if err != nil {
		return resultingShrine
	}

	/** Get Shrine perk info **/
	docShrine := doc.Find(".wikitable").First()
	docShrine.Find("td").Each(func(i int, s *goquery.Selection) {
		switch i % 3 {
		case 0:
			perk := strings.ReplaceAll(s.Text(), "\n", "")
			resultingShrine.Perks = append(resultingShrine.Perks, perk)
		case 1:
			price := strings.ReplaceAll(s.Text(), "\n", "")
			resultingShrine.Prices = append(resultingShrine.Prices, price)
		case 2:
			owner := strings.ReplaceAll(s.Text(), "\n", "")
			if len(strings.Split(owner, " ")) == 1 {
				owner = "The " + owner
			}
			resultingShrine.Owners = append(resultingShrine.Owners, owner)
		}
	})

	/** Get time until shrine resets **/
	resultingShrine.TimeUntilReset = "Shrine " + docShrine.Find("th").Last().Text()

	return resultingShrine
}

/**
Fetches perk information from https://deadbydaylight.gamepedia.com/Dead_by_Daylight_Wiki
and displays the gif icon as well as the perk's source (if there is one) and what it does.
**/
func Handle_perk(s *discordgo.Session, m *discordgo.MessageCreate, command []string) {
	if len(command) < 2 {
		s.ChannelMessageSend(m.ChannelID, "Usage: `~perk <perk name>`")
		return
	}
	requestedPerkString := format_perk(command)
	perk := scrape_perk(requestedPerkString)
	fmt.Printf("%+v\n", perk)

	// create and send response
	if perk.HTTPResponse != 200 {
		s.ChannelMessageSend(m.ChannelID, "Sorry! I couldn't find that perk :frowning:")
	} else {
		// construct complex message
		var embed discordgo.MessageEmbed
		embed.URL = perk.PageURL
		embed.Type = "rich"
		embed.Title = perk.Name
		embed.Description = perk.Description
		var thumbnail discordgo.MessageEmbedThumbnail
		thumbnail.URL = perk.IconURL
		embed.Thumbnail = &thumbnail
		var footer discordgo.MessageEmbedFooter
		footer.Text = perk.Quote
		embed.Footer = &footer
		s.ChannelMessageSendEmbed(m.ChannelID, &embed)
	}
}

/**
Checks https://deadbydaylight.gamepedia.com/Dead_by_Daylight_Wiki for the most recent shrine
post and outputs its information.
**/
func Handle_shrine(s *discordgo.Session, m *discordgo.MessageCreate) {
	shrine := scrape_shrine()
	fmt.Printf("%+v\n", shrine)

	// create and send response
	if shrine.HTTPResponse != 200 {
		s.ChannelMessageSend(m.ChannelID, "Sorry! I wasn't able to get the shrine :frowning:")
	} else {
		fmt.Println("Here's the shrine!")
		// construct embed response
		var embed discordgo.MessageEmbed
		embed.URL = "https://deadbydaylight.gamepedia.com/Shrine_of_Secrets#Current_Shrine_of_Secrets"
		embed.Type = "rich"
		embed.Title = "Current Shrine"
		var fields []*discordgo.MessageEmbedField
		var perks discordgo.MessageEmbedField
		perks.Name = "Perk"
		perks.Value = shrine.Perks[0] + "\n" + shrine.Perks[1] + "\n" + shrine.Perks[2] + "\n" + shrine.Perks[3]
		perks.Inline = true
		var costs discordgo.MessageEmbedField
		costs.Name = "Price"
		costs.Value = shrine.Prices[0] + "\n" + shrine.Prices[1] + "\n" + shrine.Prices[2] + "\n" + shrine.Prices[3]
		costs.Inline = true
		var owners discordgo.MessageEmbedField
		owners.Name = "Unique to"
		owners.Value = shrine.Owners[0] + "\n" + shrine.Owners[1] + "\n" + shrine.Owners[2] + "\n" + shrine.Owners[3]
		owners.Inline = true
		fields = append(fields, &perks, &costs, &owners)
		embed.Fields = fields
		var footer discordgo.MessageEmbedFooter
		footer.Text = shrine.TimeUntilReset
		footer.IconURL = "https://gamepedia.cursecdn.com/deadbydaylight_gamepedia_en/thumb/1/14/IconHelp_shrineOfSecrets.png/32px-IconHelp_shrineOfSecrets.png"
		embed.Footer = &footer

		// send response
		s.ChannelMessageSendEmbed(m.ChannelID, &embed)
	}
}

/**
When a new shrine tweet is received, construct a message and post it to the designated
autoshrine channel.
*/
func Handle_tweet(s *discordgo.Session, v anaconda.Tweet) {
	if strings.HasPrefix(v.Text, "This week's shrine is:") {
		// construct embed response
		var embed discordgo.MessageEmbed
		splitText := strings.Split(strings.ReplaceAll(v.FullText, "&amp;", "&"), " ")
		embed.URL = splitText[len(splitText)-1]
		embed.Type = "rich"
		embed.Title = "Latest Shrine (@DeadbyBHVR)"
		embed.Description = strings.Join(splitText[0:len(splitText)-2], " ")
		var image discordgo.MessageEmbedImage
		image.URL = v.Entities.Media[0].Media_url
		embed.Image = &image
		var thumbnail discordgo.MessageEmbedThumbnail
		thumbnail.URL = "https://pbs.twimg.com/profile_images/1281644343481249798/BLUpBkgW_400x400.png"
		embed.Thumbnail = &thumbnail

		buf, err := ioutil.ReadFile("./autoshrine_channel")
		if err != nil {
			Handle_tweet(s, v)
			return
		}

		// send response
		s.ChannelMessageSendEmbed(string(buf), &embed)
	}
}

/**
Helper function for Handle_autoshrine. Writes the new channel to file.
*/
func set_new_channel(channel string) bool {
	err := ioutil.WriteFile("./autoshrine_channel", []byte(channel), 0755)
	if err != nil {
		return false
	}
	return true
}

/**
Switches the channel that the tweet monitoring system will output to.
**/
func Handle_autoshrine(s *discordgo.Session, m *discordgo.MessageCreate, command []string) {
	if strings.HasPrefix(command[1], "<#") {
		// remove formatting
		channel := strings.ReplaceAll(command[1], "<#", "")
		channel = strings.ReplaceAll(channel, ">", "")
		if set_new_channel(channel) {
			s.ChannelMessageSend(m.ChannelID, ":slight_smile: Got it. I'll start posting the new shrines on <#"+channel+"> !")
		} else {
			s.ChannelMessageSend(m.ChannelID, ":frowning: I couldn't update the autoshrine. Try again in a moment...")
		}
	} else {
		s.ChannelMessageSend(m.ChannelID, "Usage: `~autoshrine #<channel>`")
	}
}
