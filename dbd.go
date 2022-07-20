package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode"

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

// Shrine : contains information about the current shrine pulled from the periodic-dbd-data project
type Shrine struct {
	End   string
	Perks []ShrinePerk
}

type ShrinePerk struct {
	Id          string
	Description string
	Url         string
	Img_Url     string
	Bloodpoints int
	Shards      int
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

	// special case because there is both a perk and killer named nemesis... why...
	if strings.Contains(strings.ToLower(killer), "nemesis") {
		killer = "Nemesis_T-Type"
	}
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
		logInfo("Killer Attribute Found: " + attribute)
		switch attribute {
		case "Realm":
			resultingKiller.Realm = s.Find("td").Last().Text()
		case "Power":
			resultingKiller.Power = s.Find("td").Last().Text()
		case "Power Attack Type":
			resultingKiller.PowerAttackType = s.Find("td").Last().Text()
		case "Movement Speed":
			resultingKiller.MovementSpeed = s.Find("td").Last().Text()
		case "Terror Radius":
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

	request, err := http.NewRequest(http.MethodGet, "https://raw.githubusercontent.com/cazwacki/periodic-dbd-data/master/shrine.json", nil)
	if err != nil {
		return resultingShrine
	}

	client := http.Client{
		Timeout: time.Second * 2, // Timeout after 2 seconds
	}

	res, err := client.Do(request)
	if err != nil || res.Body == nil {
		return resultingShrine
	}

	defer res.Body.Close()

	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return resultingShrine
	}

	json.Unmarshal(body, &resultingShrine)
	return resultingShrine
}

/**
Fetches survivor information from https://deadbydaylight.gamepedia.com/Dead_by_Daylight_Wiki
and displays the survivor's attributes listed in the Survivor struct.
**/
func handleSurvivor(s *discordgo.Session, m *discordgo.MessageCreate, command []string) {
	if len(command) < 2 {
		sendError(s, m, "survivor", Syntax)
		return
	}
	requestedSurvivor := strings.Join(command[1:], " ")
	requestedSurvivor = strings.ReplaceAll(strings.Title(strings.ToLower(requestedSurvivor)), " ", "_")
	survivor := scrapeSurvivor(requestedSurvivor)
	logInfo(fmt.Sprintf("%+v\n", survivor))

	// create and send response
	if survivor.Name == "" {
		sendError(s, m, "survivor", ReadParse)
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
		sendError(s, m, "killer", Syntax)
		return
	}
	requestedKiller := strings.Join(command[1:], " ")
	requestedKiller = strings.ReplaceAll(strings.Title(strings.ToLower(requestedKiller)), " ", "_")
	killer := scrapeKiller(requestedKiller)
	logInfo(fmt.Sprintf("%+v\n", killer))

	// create and send response
	if killer.Name == "" {
		sendError(s, m, "killer", ReadParse)
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
	if killer.Realm != "" {
		shortdata = append(shortdata, createField("Realm", killer.Realm, false))
	}
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
		sendError(s, m, "addon", Syntax)
		return
	}
	requestedAddonString := formatAddon(command)
	addon := scrapeAddon(requestedAddonString)
	logInfo(fmt.Sprintf("%+v\n", addon))

	// create and send response
	if addon.Name == "" {
		sendError(s, m, "addon", ReadParse)
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
		sendError(s, m, "addon", Syntax)
		return
	}
	requestedPerkString := formatPerk(command)
	perk := scrapePerk(requestedPerkString)
	logInfo(fmt.Sprintf("%+v\n", perk))

	// create and send response
	if perk.Name == "" {
		sendError(s, m, "addon", ReadParse)
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
	if len(shrine.Perks) == 0 {
		logWarning("Prices didn't correctly populate. Did the JSON structure change?")
		sendError(s, m, "shrine", ReadParse)
		return
	}

	logInfo("Retrieved the shrine")
	// construct embed response
	var embed discordgo.MessageEmbed
	embed.Type = "rich"
	embed.Title = "Current Shrine"
	var fields []*discordgo.MessageEmbedField
	perksStr := ""
	shardsStr := ""
	bloodpointsStr := ""
	for i := 0; i < len(shrine.Perks); i++ {
		perksStr += fmt.Sprintf("[%s](%s)\n", shrine.Perks[i].Id, shrine.Perks[i].Url)
		shardsStr += fmt.Sprintf("%d\n", shrine.Perks[i].Shards)
		bloodpointsStr += fmt.Sprintf("%d\n", shrine.Perks[i].Bloodpoints)
	}
	fields = append(fields, createField("Perks", perksStr, true))
	fields = append(fields, createField("Shards", shardsStr, true))
	fields = append(fields, createField("Bloodpoints", bloodpointsStr, true))
	embed.Fields = fields

	shrineEndStr, err := strconv.ParseInt(shrine.End, 10, 64)
	if err != nil {
		logError("Error parsing shrine end! " + err.Error())
		sendError(s, m, "convert", Syntax)
		return
	}

	shrineEndTime := time.Unix(shrineEndStr, 0)

	// convert fetched time into utc timestamp for discord
	discordTimestamp := "2006-01-02T15:04:05.999Z"
	utc, err := time.LoadLocation("UTC")
	if err != nil {
		logError("Error loading UTC! " + err.Error())
		sendError(s, m, "convert", Internal)
		return
	}

	adjustedTime := shrineEndTime.In(utc)
	logInfo(adjustedTime.Format(discordTimestamp))

	var footer discordgo.MessageEmbedFooter
	footer.Text = "Shrine refreshes on"
	embed.Timestamp = adjustedTime.Format(discordTimestamp)
	embed.Footer = &footer
	footer.IconURL = "https://gamepedia.cursecdn.com/deadbydaylight_gamepedia_en/thumb/1/14/IconHelp_shrineOfSecrets.png/32px-IconHelp_shrineOfSecrets.png"
	embed.Footer = &footer

	// send response
	_, err = s.ChannelMessageSendEmbed(m.ChannelID, &embed)
	if err != nil {
		logError("Failed to send shrine embed! " + err.Error())
		return
	}
}
