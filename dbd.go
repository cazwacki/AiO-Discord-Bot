package main

import (
	"fmt"
	"io/ioutil"
	"net/url"
	"regexp"
	"strings"
	"unicode"

	"github.com/ChimeraCoder/anaconda"
	"github.com/PuerkitoBio/goquery"
	"github.com/bwmarrin/discordgo"
)

type Survivor struct {
	Name     string
	IconURL  string
	Role     string
	Overview string
	Perks    []string
	PerkURLs []string
	PageURL  string
}

type Killer struct {
	Name            string
	IconURL         string
	Realm           string
	Power           string
	PowerAttackType string
	MovementSpeed   string
	TerrorRadius    string
	Height          string
	Overview        string
	Perks           []string
	PerkURLs        []string
	PageURL         string
}

type Addon struct {
	Name        string
	IconURL     string
	Description string
	PageURL     string
}

// Perk : contains information about the perk pulled from the wiki
type Perk struct {
	Name        string
	IconURL     string
	Description string
	Quote       string
	PageURL     string
}

// Shrine : contains information about the current shrine pulled from the wiki
type Shrine struct {
	Prices         []string
	Perks          []string
	Owners         []string
	TimeUntilReset string
}

/**
Helper function for handle_addon and should be used before scrape_addon
to ensure the string is appropriately formatted such that it can be used
as the end of the URL query to https://deadbydaylight.gamepedia.com/.
*/
func formatAddon(command []string) string {
	specialWords := " of on up brand outs "
	for i := 1; i < len(command); i++ {
		words := strings.Split(command[i], "-")
		for j := 0; j < len(words); j++ {
			if !strings.Contains(specialWords, words[j]) {
				tmp := []rune(words[j])
				tmp[0] = unicode.ToUpper(tmp[0])
				words[j] = string(tmp)
			}
		}
		command[i] = strings.Join(words, "-")
	}
	addon := strings.Join(command[1:], "_")
	addon = strings.Replace(addon, "_And", "_&", 1)
	addon = url.QueryEscape(addon)
	return addon
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
			words := strings.Split(command[i], "-")
			for j := 0; j < len(words); j++ {
				tmp := []rune(words[j])
				tmp[0] = unicode.ToUpper(tmp[0])
				words[j] = string(tmp)
			}
			command[i] = strings.Join(words, "-")
		}
	}
	perk := strings.Join(command[1:], "_")
	perk = strings.Replace(perk, "_And", "_&", 1)
	perk = url.QueryEscape(perk)
	return perk
}

/**
Helper function for handle_survivor. Scrapes HTML from the respective
page on https://deadbydaylight.gamepedia.com/ and returns the
desired information in the Survivor struct created above.
*/
func scrapeSurvivor(survivor string) Survivor {
	var resultingSurvivor Survivor

	resultingSurvivor.PageURL = "https://deadbydaylight.gamepedia.com/" + survivor

	// Request the HTML page.
	doc := loadPage(resultingSurvivor.PageURL)

	if doc == nil {
		return resultingSurvivor
	}

	// get name
	docName := doc.Find("#firstHeading").First()
	resultingSurvivor.Name = strings.TrimSpace(docName.Text())

	docData := doc.Find(".infoboxtable").First()

	// get icon URL
	currentSrc := docData.Find("img").First().AttrOr("src", "nil")
	logInfo(currentSrc)
	if strings.Contains(currentSrc, ".png") {
		resultingSurvivor.IconURL = currentSrc
	}

	currentSrc = docData.Find("img").First().AttrOr("data-src", "nil")
	logInfo(currentSrc)
	if strings.Contains(currentSrc, ".png") && resultingSurvivor.IconURL == "" {
		resultingSurvivor.IconURL = currentSrc
	}

	// get short data
	docData.Find("tr").Each(func(i int, s *goquery.Selection) {
		attribute := s.Find("td").First().Text()
		switch attribute {
		case "Role":
			resultingSurvivor.Role = s.Find("td").Last().Text()
		default:
			logWarning("Skipping " + attribute + " while scraping killer")
		}
	})

	// get overview
	resultingSurvivor.Overview = doc.Find("#Overview").First().Parent().Next().Text()
	if resultingSurvivor.Overview == "" {
		resultingSurvivor.Overview = doc.Find("#Overview").First().Parent().Next().Next().Text()
	}

	// get perks
	docPerks := doc.Find(".wikitable").First()
	docPerks.Find("tr").Each(func(i int, s *goquery.Selection) {
		resultingSurvivor.Perks = append(resultingSurvivor.Perks, s.Find("th").Last().Text())
		resultingSurvivor.PerkURLs = append(resultingSurvivor.PerkURLs, "https://deadbydaylight.gamepedia.com"+s.Find("th").Last().Find("a").AttrOr("href", "nil"))
	})

	return resultingSurvivor
}

/**
Helper function for handle_killer. Scrapes HTML from the respective
page on https://deadbydaylight.gamepedia.com/ and returns the
desired information in the Killer struct created above.
*/
func scrapeKiller(killer string) Killer {
	var resultingKiller Killer

	resultingKiller.PageURL = "https://deadbydaylight.gamepedia.com/" + killer

	// Request the HTML page.
	doc := loadPage(resultingKiller.PageURL)

	if doc == nil {
		return resultingKiller
	}

	// get name
	docName := doc.Find("#firstHeading").First()
	resultingKiller.Name = strings.TrimSpace(docName.Text())

	docData := doc.Find(".infoboxtable").First()

	// get icon URL
	currentSrc := docData.Find("img").First().AttrOr("src", "nil")
	logInfo(currentSrc)
	if strings.Contains(currentSrc, ".png") {
		resultingKiller.IconURL = currentSrc
	}

	currentSrc = docData.Find("img").First().AttrOr("data-src", "nil")
	logInfo(currentSrc)
	if strings.Contains(currentSrc, ".png") && resultingKiller.IconURL == "" {
		resultingKiller.IconURL = currentSrc
	}

	// get short data
	docData.Find("tr").Each(func(i int, s *goquery.Selection) {
		attribute := s.Find("td").First().Text()
		switch attribute {
		case "Realm":
			resultingKiller.Realm = s.Find("td").Last().Text()
		case "Power":
			resultingKiller.Power = s.Find("td").Last().Text()
		case "Power Attack Type":
			resultingKiller.PowerAttackType = s.Find("td").Last().Text()
		case "Movement speed ":
			resultingKiller.MovementSpeed = s.Find("td").Last().Text()
		case "Terror Radius ":
			resultingKiller.TerrorRadius = s.Find("td").Last().Text()
		case "Height ":
			resultingKiller.Height = s.Find("td").Last().Text()
		default:
			logWarning("Skipping " + attribute + " while scraping killer")
		}
	})

	if resultingKiller.PowerAttackType == "" {
		resultingKiller.PowerAttackType = "Basic Attack"
	}

	// get overview
	resultingKiller.Overview = doc.Find("#Overview").First().Parent().Next().Text()
	if resultingKiller.Overview == "" {
		resultingKiller.Overview = doc.Find("#Overview").First().Parent().Next().Next().Text()
	}

	// get perks
	docPerks := doc.Find(".wikitable").First()
	docPerks.Find("tr").Each(func(i int, s *goquery.Selection) {
		resultingKiller.Perks = append(resultingKiller.Perks, s.Find("th").Last().Text())
		resultingKiller.PerkURLs = append(resultingKiller.PerkURLs, "https://deadbydaylight.gamepedia.com"+s.Find("th").Last().Find("a").AttrOr("href", "nil"))
	})

	return resultingKiller
}

/**
Helper function for handle_addon. Scrapes HTML from the respective
page on https://deadbydaylight.gamepedia.com/ and returns the
desired information in the Addon struct created above.
*/
func scrapeAddon(addon string) Addon {
	var resultingAddon Addon

	resultingAddon.PageURL = "https://deadbydaylight.gamepedia.com/" + addon

	// Request the HTML page.
	doc := loadPage(resultingAddon.PageURL)

	if doc == nil {
		return resultingAddon
	}

	// get name
	docName := doc.Find("#firstHeading").First()
	resultingAddon.Name = docName.Text()

	docData := doc.Find(".wikitable").First().Find("tr").Last()

	// get icon URL
	currentSrc := docData.Find("img").First().AttrOr("src", "nil")
	if strings.Contains(currentSrc, ".png") {
		resultingAddon.IconURL = currentSrc
	}

	// get description
	docDescription := docData.Find("td").First().Text()

	// remove impurities
	docDescription = strings.ReplaceAll(docDescription, " .", ".")
	docDescription = strings.ReplaceAll(docDescription, "  ", " ")
	docDescription = strings.ReplaceAll(docDescription, " %", "%")
	docDescription = strings.ReplaceAll(docDescription, "\n", "\n\n")

	resultingAddon.Description = docDescription

	return resultingAddon
}

/**
Helper function for handle_perk. Scrapes HTML from the respective
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
	docName := doc.Find("#firstHeading").First()
	resultingPerk.Name = docName.Text()

	/** Get Description **/
	docDesc := doc.Find(".wikitable").First().Find(".formattedPerkDesc").Last()

	// docDescText := docDesc.Text()
	html, err := docDesc.Html()
	if err != nil {
		resultingPerk.Name = ""
		return resultingPerk
	}
	r := regexp.MustCompile(`<.*?>`)
	docDescText := r.ReplaceAllString(html, "")
	// strings.ReplaceAll(docDesc.Text(), "\n", " ")

	// remove impurities
	description := strings.ReplaceAll(docDescText, " .", ".")
	description = strings.ReplaceAll(description, "&#34;", "\"")
	description = strings.ReplaceAll(description, "&#39;", "'")
	description = strings.ReplaceAll(description, ". ", ".\n")
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
	docShrine := doc.Find(".sosTable").First()
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
Fetches survivor information from https://deadbydaylight.gamepedia.com/Dead_by_Daylight_Wiki
and displays the survivor's attributes listed in the Survivor struct.
**/
func handleSurvivor(s *discordgo.Session, m *discordgo.MessageCreate, command []string) {
	if len(command) < 2 {
		s.ChannelMessageSend(m.ChannelID, "Usage: `~survivor <survivor name>`")
		return
	}
	requestedSurvivor := strings.Join(command[1:], " ")
	requestedSurvivor = strings.ReplaceAll(strings.Title(strings.ToLower(requestedSurvivor)), " ", "_")
	survivor := scrapeSurvivor(requestedSurvivor)
	logInfo(fmt.Sprintf("%+v\n", survivor))

	// create and send response
	if survivor.Name == "" {
		s.ChannelMessageSend(m.ChannelID, "Sorry! I couldn't find that survivor :frowning:")
		return
	}

	// construct embed message
	var embed discordgo.MessageEmbed
	embed.URL = survivor.PageURL
	embed.Type = "rich"
	embed.Title = survivor.Name
	embed.Description = survivor.Overview
	var shortdata []*discordgo.MessageEmbedField
	shortdata = append(shortdata, createField("Role", survivor.Role, false))
	perkText := ""
	for i := 0; i < 3; i++ {
		perkText += fmt.Sprintf("[%s](%s)\n", strings.TrimSpace(survivor.Perks[i]), strings.TrimSpace(survivor.PerkURLs[i]))
	}
	logInfo(perkText)
	shortdata = append(shortdata, createField("Teachable Perks", perkText, false))
	embed.Fields = shortdata
	var thumbnail discordgo.MessageEmbedThumbnail
	thumbnail.URL = survivor.IconURL
	embed.Thumbnail = &thumbnail
	_, err := s.ChannelMessageSendEmbed(m.ChannelID, &embed)
	if err != nil {
		logError("Failed to send message embed. " + err.Error())
	}
}

/**
Fetches killer information from https://deadbydaylight.gamepedia.com/Dead_by_Daylight_Wiki
and displays the killer's attributes listed in the Killer struct.
**/
func handleKiller(s *discordgo.Session, m *discordgo.MessageCreate, command []string) {
	if len(command) < 2 {
		s.ChannelMessageSend(m.ChannelID, "Usage: `~killer <killer name>`")
		return
	}
	requestedKiller := strings.Join(command[1:], " ")
	requestedKiller = strings.ReplaceAll(strings.Title(strings.ToLower(requestedKiller)), " ", "_")
	killer := scrapeKiller(requestedKiller)
	logInfo(fmt.Sprintf("%+v\n", killer))

	// create and send response
	if killer.Name == "" {
		s.ChannelMessageSend(m.ChannelID, "Sorry! I couldn't find that killer :frowning:")
		return
	}

	// construct embed message
	var embed discordgo.MessageEmbed
	embed.URL = killer.PageURL
	embed.Type = "rich"
	embed.Title = killer.Name
	embed.Description = killer.Overview
	var shortdata []*discordgo.MessageEmbedField
	shortdata = append(shortdata, createField("Power", killer.Power, false))
	shortdata = append(shortdata, createField("Movement Speed", killer.MovementSpeed, false))
	shortdata = append(shortdata, createField("Terror Radius", killer.TerrorRadius, false))
	shortdata = append(shortdata, createField("Height", killer.Height, false))
	shortdata = append(shortdata, createField("Power Attack Type", killer.PowerAttackType, false))
	shortdata = append(shortdata, createField("Realm", killer.Realm, false))
	perkText := ""
	for i := 0; i < 3; i++ {
		perkText += fmt.Sprintf("[%s](%s)\n", strings.TrimSpace(killer.Perks[i]), strings.TrimSpace(killer.PerkURLs[i]))
	}
	logInfo(perkText)
	shortdata = append(shortdata, createField("Teachable Perks", perkText, false))
	embed.Fields = shortdata
	var thumbnail discordgo.MessageEmbedThumbnail
	thumbnail.URL = killer.IconURL
	embed.Thumbnail = &thumbnail
	_, err := s.ChannelMessageSendEmbed(m.ChannelID, &embed)
	if err != nil {
		logError("Failed to send message embed. " + err.Error())
	}
}

/**
Fetches addon information from https://deadbydaylight.gamepedia.com/Dead_by_Daylight_Wiki
and displays the png icon as well as the add-on's function.
**/
func handleAddon(s *discordgo.Session, m *discordgo.MessageCreate, command []string) {
	if len(command) < 2 {
		s.ChannelMessageSend(m.ChannelID, "Usage: `~addon <addon name>`")
		return
	}
	requestedAddonString := formatAddon(command)
	addon := scrapeAddon(requestedAddonString)
	logInfo(fmt.Sprintf("%+v\n", addon))

	// create and send response
	if addon.Name == "" {
		s.ChannelMessageSend(m.ChannelID, "Sorry! I couldn't find that add-on :frowning:")
		return
	}
	// construct embed message
	var embed discordgo.MessageEmbed
	embed.URL = addon.PageURL
	embed.Type = "rich"
	embed.Title = addon.Name
	embed.Description = addon.Description
	var thumbnail discordgo.MessageEmbedThumbnail
	thumbnail.URL = addon.IconURL
	embed.Thumbnail = &thumbnail
	_, err := s.ChannelMessageSendEmbed(m.ChannelID, &embed)
	if err != nil {
		logError("Failed to send message embed. " + err.Error())
	}
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
	logInfo(fmt.Sprintf("%+v\n", perk))

	// create and send response
	if perk.Name == "" {
		s.ChannelMessageSend(m.ChannelID, "Sorry! I couldn't find that perk :frowning:")
		return
	}
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

/**
Checks https://deadbydaylight.gamepedia.com/Dead_by_Daylight_Wiki for the most recent shrine
post and outputs its information.
**/
func handleShrine(s *discordgo.Session, m *discordgo.MessageCreate, command []string) {
	logInfo(strings.Join(command, " "))
	shrine := scrapeShrine()
	logInfo(fmt.Sprintf("%+v\n", shrine))

	// create and send response
	if len(shrine.Prices) == 0 {
		logWarning("Prices didn't correctly populate. Did the website design change?")
		_, err := s.ChannelMessageSend(m.ChannelID, "Sorry! I wasn't able to get the shrine :frowning:")
		if err != nil {
			logError("Failed to send 'failed to retrieve shrine' message! " + err.Error())
			return
		}
		return
	}

	logInfo("Retrieved the shrine")
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
	_, err := s.ChannelMessageSendEmbed(m.ChannelID, &embed)
	if err != nil {
		logError("Failed to send shrine embed! " + err.Error())
		return
	}
	logSuccess("Sent shrine embed")
}

/**
When a new shrine tweet is received, construct a message and post it to the designated
autoshrine channel.
*/
func handleTweet(s *discordgo.Session, v anaconda.Tweet) {
	if strings.HasPrefix(v.Text, "This week's shrine is") && v.User.Id == 4850837842 {
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

		// attempt to read channel identity from file
		buf, err := ioutil.ReadFile("./autoshrine_channel")
		if err != nil {
			logError("Unable to read file for autoshrine channel. " + err.Error())
			handleTweet(s, v)
			return
		}

		// send response
		_, err = s.ChannelMessageSendEmbed(string(buf), &embed)
		if err != nil {
			logError("Failed to send tweet embed! " + err.Error())
			return
		}
	}
}

/**
Helper function for Handle_autoshrine. Writes the new channel to file.
*/
func setNewChannel(channel string) bool {
	err := ioutil.WriteFile("./autoshrine_channel", []byte(channel), 0644)
	if err != nil {
		logError("Failed to write new autoshrine! " + err.Error())
		return false
	}
	return true
}

/**
Switches the channel that the tweet monitoring system will output to.
**/
func handleAutoshrine(s *discordgo.Session, m *discordgo.MessageCreate, command []string) {
	// correct usage of autoshrine?
	if len(command) != 2 {
		logInfo("User passed in incorrect number of arguments")
		_, err := s.ChannelMessageSend(m.ChannelID, "Usage: `~autoshrine #<channel>`")
		if err != nil {
			logError("Failed to send usage message! " + err.Error())
		}
		return
	}

	// is the second field a channel?
	if !strings.HasPrefix(command[1], "<#") {
		logInfo("User passed in invalid channel")
		_, err := s.ChannelMessageSend(m.ChannelID, "Please pass in a valid channel.")
		if err != nil {
			logError("Failed to send invalid channel message! " + err.Error())
			return
		}
		return
	}

	// remove formatting
	channel := strings.ReplaceAll(command[1], "<#", "")
	channel = strings.ReplaceAll(channel, ">", "")
	if setNewChannel(channel) {
		_, err := s.ChannelMessageSend(m.ChannelID, ":slight_smile: Got it. I'll start posting the new shrines on <#"+channel+"> !")
		if err != nil {
			logError("Failed to send successful update message! " + err.Error())
			return
		}
	} else {
		_, err := s.ChannelMessageSend(m.ChannelID, ":frowning: I couldn't update the autoshrine. Try again in a moment...")
		if err != nil {
			logError("Failed to send failed update message! " + err.Error())
			return
		}
	}
	logSuccess("Updated autoshrine")
}
