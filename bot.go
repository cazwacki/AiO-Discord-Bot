package main

import (
	"fmt"
	"net/url"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/ChimeraCoder/anaconda"
	"github.com/bwmarrin/discordgo"
)

var start time.Time
var prodMode bool
var globalImageSet []*ImageSet
var prefix string
var commandList map[string]command

type handler func(*discordgo.Session, *discordgo.MessageCreate, []string)

type command struct {
	invoke_format string
	description   string
	handle        handler
}

func appendToGlobalImageSet(s *discordgo.Session, newset ImageSet) {
	globalImageSet = append(globalImageSet, &newset)
	fmt.Println("Global Image Set:")
	fmt.Println(globalImageSet)

	time.Sleep(30 * time.Minute)

	for i, set := range globalImageSet {
		if &newset == set {
			tmpSet := globalImageSet[0]
			globalImageSet[0] = globalImageSet[i]
			globalImageSet[i] = tmpSet
			globalImageSet = globalImageSet[1:]
			s.MessageReactionsRemoveAll(newset.Message.ChannelID, newset.Message.ID)
		}
	}
}

/**
Initialize command information and prefix
*/
func initCommandInfo() {
	prefix = "~"
	commandList = map[string]command{
		"uptime":     {"uptime", "Reports the bot's current uptime.", handleUptime},
		"shutdown":   {"shutdown", "Shuts the bot down cleanly. Note that if the bot is deployed on an automatic service such as Heroku it will automatically restart.", handleShutdown},
		"invite":     {"invite", "Generates a server invitation valid for 24 hours.", handleInvite},
		"profile":    {"profile @user", "Shows the profile image of a user in an embed.", handleProfile},
		"nick":       {"nick @user <nickname>", "Renames the specified user to the provided nickname.", handleNickname},
		"kick":       {"kick @user (reason: optional)", "Kicks the specified user from the server.", handleKick},
		"ban":        {"ban @user (reason:optional)", "Bans the specified user from the server.", handleBan},
		"mv":         {"mv <number> <#channel>", "Moves the last <number> messages from the channel it is invoked in and moves them to <#channel>.", handleMove},
		"cp":         {"cp <number> <#channel>", "Copies the last <number> messages from the channel it is invoked in and pastes them to <#channel>.", handleCopy},
		"purge":      {"purge <number>", "Removes the <number> most recent messages from the channel.", handlePurge},
		"define":     {"define <word/phrase>", "Returns a definition of the word/phrase if it is available.", handleDefine},
		"google":     {"google <word/phrase>", "Returns the first five google results returned from the query.", handleGoogle},
		"image":      {"image <word/phrase>", "Returns the first image from Google Images.", handleImage},
		"perk":       {"perk <perk name>", "Returns the description of the specified Dead by Daylight perk.", handlePerk},
		"shrine":     {"shrine", "Returns the current shrine according to the Dead by Daylight Wiki.", handleShrine},
		"autoshrine": {"autoshrine <#channel>", "Changes the channel where Tweets about the newest shrine from @DeadbyBHVR are posted.", handleAutoshrine},
		"help":       {"help", "Returns how to use each of the commands the bot has available.", handleHelp},
		"wiki":       {"wiki <word/phrase>", "Returns the extract from the corresponding Wikipedia page.", handleWiki},
	}
}

func runBot(token string) {
	/** Open Connection to Discord **/
	if os.Getenv("PROD_MODE") == "true" {
		prodMode = true
	}
	start = time.Now()

	// initialize bot
	dg, err := discordgo.New("Bot " + token)
	if err != nil {
		fmt.Println("Error creating discord session")
		return
	}

	// add listeners
	dg.AddHandler(messageCreate)
	dg.AddHandler(messageReactionAdd)

	// open connection to discord
	err = dg.Open()
	if err != nil {
		fmt.Println("Error opening connection,", err)
		return
	}

	initCommandInfo()

	/** Open Connection to Twitter **/
	anaconda.SetConsumerKey(os.Getenv("TWITTER_API_KEY"))
	anaconda.SetConsumerSecret(os.Getenv("TWITTER_API_SECRET"))
	api := anaconda.NewTwitterApi(os.Getenv("TWITTER_TOKEN"), os.Getenv("TWITTER_TOKEN_SECRET"))
	go runTwitterLoop(api, dg)

	// Wait here until CTRL-C or other term signal is received.
	fmt.Println("Bot is now running.  Press CTRL-C to exit.")
	sc := make(chan os.Signal, 1)
	signal.Notify(sc, syscall.SIGINT, syscall.SIGTERM, os.Interrupt, os.Kill)
	<-sc

	// Cleanly close down the Discord session and Twitter connection.
	dg.Close()
	api.Close()
}

/**
Opens a stream looking for new tweets from @DeadbyBHVR, who posts the weekly
shrine on Twitter.
*/
func runTwitterLoop(api *anaconda.TwitterApi, dg *discordgo.Session) {
	fmt.Println("Starting...")
	v := url.Values{}
	v.Set("follow", "4850837842") // @DeadbyBHVR is 4850837842
	v.Set("track", "shrine")
	s := api.PublicStreamFilter(v)
	for t := range s.C {
		switch v := t.(type) {
		case anaconda.Tweet:
			handleTweet(dg, v)
		}
	}
}

/**
Creates an embed displaying all the potential commands and their functions.
*/
func handleHelp(s *discordgo.Session, m *discordgo.MessageCreate, command []string) {
	// return all current commands and what they do
	var embed discordgo.MessageEmbed
	embed.Type = "rich"

	embed.Title = "❓ How to Use ZawackiBot ❓"

	// add a cute thumbnail
	var thumbnail discordgo.MessageEmbedThumbnail
	thumbnail.URL = "https://img.pngio.com/robot-icon-of-flat-style-available-in-svg-png-eps-ai-icon-robot-icon-png-256_256.png"
	embed.Thumbnail = &thumbnail

	// add all commands to the embed as a set of fields that are not inline
	embed.Description = "The commands are listed on the [Github page](https://github.com/cazwacki/PersonalDiscordBot#commands) for this bot now!"

	// self-credit + github profile picture
	var footer discordgo.MessageEmbedFooter
	footer.Text = "Created by Charles Zawacki; Written in Go"
	footer.IconURL = "https://avatars0.githubusercontent.com/u/44577941?s=460&u=4eb7b9ff5410be189eea9863c33916c805dbd2b2&v=4"
	embed.Footer = &footer

	// send response
	s.ChannelMessageSendEmbed(m.ChannelID, &embed)

}

/**
Handler function when the discord session detects a message is created in
a channel that the bot has access to.
*/
func messageCreate(s *discordgo.Session, m *discordgo.MessageCreate) {
	// Ignore my testing channel
	if prodMode && m.ChannelID == "739852388264968243" {
		return
	}
	// Ignore all messages created by the bot itself as well as DMs
	if m.Author.ID == s.State.User.ID || m.GuildID == "" {
		return
	}

	parsedCommand := strings.Split(m.Content, " ")
	if !strings.HasPrefix(parsedCommand[0], prefix) {
		return
	}
	invoke_word := strings.TrimPrefix(parsedCommand[0], prefix)

	// get the command information based on the invoke word
	if validCommand, ok := commandList[invoke_word]; ok {
		// special case: needs to pass in starting time
		if parsedCommand[0] == prefix+"uptime" {
			parsedCommand = []string{start.Format("2006-01-02 15:04:05.999999999 -0700 MST")}
		}
		go validCommand.handle(s, m, parsedCommand)
	}
}

/**
Used to handle scrolling through images given from ~image,
but can and may be used to handle other reactions in the future
*/
func messageReactionAdd(s *discordgo.Session, m *discordgo.MessageReactionAdd) {
	// Ignore all messages created by the bot itself as well as DMs
	if m.UserID == s.State.User.ID {
		return
	}
	for i, set := range globalImageSet {
		if set.Message.ID == m.MessageID {
			if m.Emoji.Name == "⬅️" || m.Emoji.Name == "➡️" || m.Emoji.Name == "⏹️" {
				/*
					1. Remove the reaction the user made
					2. Switch the image if the index allows
					3. Update the index in the set
				*/
				if m.Emoji.Name == "⏹️" {
					// remove reactions and remove from list
					tmpSet := globalImageSet[0]
					globalImageSet[0] = globalImageSet[i]
					globalImageSet[i] = tmpSet
					globalImageSet = globalImageSet[1:]
					s.MessageReactionsRemoveAll(m.ChannelID, m.MessageID)
				} else {
					// craft response and send
					var embed discordgo.MessageEmbed
					embed.Type = "rich"
					embed.Title = "Image Results for \"" + set.Query + "\""

					fmt.Printf("Index before: %d\n", set.Index)
					if m.Emoji.Name == "⬅️" {
						if set.Index != 0 {
							set.Index--
						}
					} else if m.Emoji.Name == "➡️" {
						if set.Index != len(set.Images)-1 {
							set.Index++
						}
					}
					fmt.Printf("Index after: %d\n", set.Index)

					var image discordgo.MessageEmbedImage
					image.URL = set.Images[set.Index]
					embed.Image = &image
					var footer discordgo.MessageEmbedFooter
					footer.Text = fmt.Sprintf("Image %d of %d", set.Index+1, len(set.Images))
					footer.IconURL = "https://cdn4.iconfinder.com/data/icons/new-google-logo-2015/400/new-google-favicon-512.png"
					embed.Footer = &footer
					s.ChannelMessageEditEmbed(m.ChannelID, m.MessageID, &embed)
					s.MessageReactionRemove(m.ChannelID, m.MessageID, m.Emoji.Name, m.UserID)
				}
			}
		}
	}
}
