package main

import (
	"fmt"
	"io/ioutil"
	"net/url"
	"strings"
	"unicode"

	"github.com/ChimeraCoder/anaconda"
	"github.com/PuerkitoBio/goquery"
	"github.com/bwmarrin/discordgo"
)

type Perk struct {
	Name        string
	IconURL     string
	Description string
	Quote       string
	PageURL     string
}

type Shrine struct {
	Prices         []string
	Perks          []string
	Owners         []string
	TimeUntilReset string
}

/**
Helper function for Handle_perk and should be used before scrape_perk.
Formats the perk provided in the command such that it can be used as
the end of the URL query to https://deadbydaylight.gamepedia.com/.
*/
func formatPerk(command []string) string {
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
func scrapePerk(perk string) Perk {
	var resultingPerk Perk

	resultingPerk.PageURL = "https://deadbydaylight.gamepedia.com/" + perk

	// Request the HTML page.
	doc := loadPage(resultingPerk.PageURL)

	if doc == nil {
		return resultingPerk
	}

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
	doc.Find(".wikitable").First().Find("img").Each(func(i int, s *goquery.Selection) {
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
func scrapeShrine() Shrine {
	var resultingShrine Shrine
	// Request the HTML page.
	doc := loadPage("https://deadbydaylight.gamepedia.com/Shrine_of_Secrets")

	if doc == nil {
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
func handlePerk(s *discordgo.Session, m *discordgo.MessageCreate, command []string) {
	if len(command) < 2 {
		s.ChannelMessageSend(m.ChannelID, "Usage: `~perk <perk name>`")
		return
	}
	requestedPerkString := formatPerk(command)
	perk := scrapePerk(requestedPerkString)
	fmt.Printf("%+v\n", perk)

	// create and send response
	if perk.Name == "" {
		s.ChannelMessageSend(m.ChannelID, "Sorry! I couldn't find that perk :frowning:")
	} else {
		// construct embed message
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
func handleShrine(s *discordgo.Session, m *discordgo.MessageCreate) {
	shrine := scrapeShrine()
	fmt.Printf("%+v\n", shrine)

	// create and send response
	if len(shrine.Prices) == 0 {
		s.ChannelMessageSend(m.ChannelID, "Sorry! I wasn't able to get the shrine :frowning:")
	} else {
		fmt.Println("Here's the shrine!")
		// construct embed response
		var embed discordgo.MessageEmbed
		embed.URL = "https://deadbydaylight.gamepedia.com/Shrine_of_Secrets#Current_Shrine_of_Secrets"
		embed.Type = "rich"
		embed.Title = "Current Shrine"
		var fields []*discordgo.MessageEmbedField
		fields = append(fields, createField("Perk", shrine.Perks[0]+"\n"+shrine.Perks[1]+"\n"+shrine.Perks[2]+"\n"+shrine.Perks[3], true))
		fields = append(fields, createField("Price", shrine.Prices[0]+"\n"+shrine.Prices[1]+"\n"+shrine.Prices[2]+"\n"+shrine.Prices[3], true))
		fields = append(fields, createField("Unique to", shrine.Owners[0]+"\n"+shrine.Owners[1]+"\n"+shrine.Owners[2]+"\n"+shrine.Owners[3], true))
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
func handleTweet(s *discordgo.Session, v anaconda.Tweet) {
	if strings.HasPrefix(v.Text, "This week's shrine is:") {
		// construct embed response
		var embed discordgo.MessageEmbed
		splitText := strings.Split(strings.ReplaceAll(v.FullText, "&amp;", "&"), " ")
		embed.URL = splitText[len(splitText)-1]
		embed.Type = "rich"
		embed.Title = "Latest Shrine (@DeadbyBHVR)"
		embed.Description = strings.Join(splitText[0:len(splitText)-1], " ")
		var image discordgo.MessageEmbedImage
		image.URL = v.Entities.Media[0].Media_url
		embed.Image = &image
		var thumbnail discordgo.MessageEmbedThumbnail
		thumbnail.URL = "https://pbs.twimg.com/profile_images/1281644343481249798/BLUpBkgW_400x400.png"
		embed.Thumbnail = &thumbnail

		buf, err := ioutil.ReadFile("./autoshrine_channel")
		if err != nil {
			handleTweet(s, v)
			return
		}

		// send response
		s.ChannelMessageSendEmbed(string(buf), &embed)
	}
}

/**
Helper function for Handle_autoshrine. Writes the new channel to file.
*/
func setNewChannel(channel string) bool {
	err := ioutil.WriteFile("./autoshrine_channel", []byte(channel), 0644)
	if err != nil {
		return false
	}
	return true
}

/**
Switches the channel that the tweet monitoring system will output to.
**/
func handleAutoshrine(s *discordgo.Session, m *discordgo.MessageCreate, command []string) {
	if len(command) == 2 {
		if strings.HasPrefix(command[1], "<#") {
			// remove formatting
			channel := strings.ReplaceAll(command[1], "<#", "")
			channel = strings.ReplaceAll(channel, ">", "")
			if setNewChannel(channel) {
				s.ChannelMessageSend(m.ChannelID, ":slight_smile: Got it. I'll start posting the new shrines on <#"+channel+"> !")
			} else {
				s.ChannelMessageSend(m.ChannelID, ":frowning: I couldn't update the autoshrine. Try again in a moment...")
			}
		} else {
			s.ChannelMessageSend(m.ChannelID, "Usage: `~autoshrine #<channel>`")
		}
	} else {
		s.ChannelMessageSend(m.ChannelID, "Usage: `~autoshrine #<channel>`")
	}
}
