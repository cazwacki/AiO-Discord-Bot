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
var prod_mode bool
var globalImageSet []*ImageSet

func appendToGlobalImageSet(newset ImageSet) {
	globalImageSet = append(globalImageSet, &newset)
	fmt.Println("Global Image Set:")
	fmt.Println(globalImageSet)
}

func Run_bot(token string) {

	/** Open Connection to Discord **/
	if os.Getenv("PROD_MODE") == "true" {
		prod_mode = true
	}
	start = time.Now()

	dg, err := discordgo.New("Bot " + token)
	if err != nil {
		fmt.Println("Error creating discord session")
		return
	}

	dg.AddHandler(messageCreate)
	dg.AddHandler(messageReactionAdd)

	err = dg.Open()
	if err != nil {
		fmt.Println("Error opening connection,", err)
		return
	}

	/** Open Connection to Twitter **/
	anaconda.SetConsumerKey(os.Getenv("TWITTER_API_KEY"))
	anaconda.SetConsumerSecret(os.Getenv("TWITTER_API_SECRET"))
	api := anaconda.NewTwitterApi(os.Getenv("TWITTER_TOKEN"), os.Getenv("TWITTER_TOKEN_SECRET"))
	go run_twitter_loop(api, dg)

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
func run_twitter_loop(api *anaconda.TwitterApi, dg *discordgo.Session) {
	fmt.Println("Starting...")
	v := url.Values{}
	v.Set("follow", "4850837842") // @DeadbyBHVR is 4850837842
	v.Set("track", "shrine")
	s := api.PublicStreamFilter(v)
	for t := range s.C {
		switch v := t.(type) {
		case anaconda.Tweet:
			Handle_tweet(dg, v)
		}
	}
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
	commands = append(commands, createField("~uptime", "Reports the bot's current uptime.", false))
	commands = append(commands, createField("~shutdown", "Shuts the bot down cleanly. Note that if the bot is deployed on an automatic service such as Heroku it will automatically restart.", false))
	commands = append(commands, createField("~invite", "Generates a server invitation valid for 24 hours.", false))
	commands = append(commands, createField("~nick @user <nickname>", "Renames the specified user to the provided nickname.", false))
	commands = append(commands, createField("~kick @user (reason: optional)", "Kicks the specified user from the server.", false))
	commands = append(commands, createField("~ban @user (reason:optional)", "Bans the specified user from the server.", false))
	commands = append(commands, createField("~perk <perk name>", "Returns the description of the specified Dead by Daylight perk.", false))
	commands = append(commands, createField("~shrine", "Returns the current shrine according to the Dead by Daylight Wiki.", false))
	commands = append(commands, createField("~autoshrine <#channel>", "Changes the channel where Tweets about the newest shrine from @DeadbyBHVR are posted.", false))
	commands = append(commands, createField("~define <word/phrase>", "Returns a definition of the word/phrase if it is available.", false))
	commands = append(commands, createField("~google <word/phrase>", "Returns the first five google results returned from the query.", false))
	commands = append(commands, createField("~image <word/phrase>", "Returns the first image from Google Images.", false))
	commands = append(commands, createField("~help", "Returns how to use each of the commands the bot has available.", false))
	embed.Fields = commands
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

TODO: Make a map of {
	command
	description/help
	handler_function
}
Then this "code" would be:
dispatcher = commands[parsedCommand[0]];
if (dispatcher != null) {
	dispatcher(s, m, parsedCommand);
}

Similarly, the help function above would be really easy
commands.forEach() {
	add command.description to the result message.
}
*/
func messageCreate(s *discordgo.Session, m *discordgo.MessageCreate) {
	// Ignore my testing channel
	if prod_mode && m.ChannelID == "739852388264968243" {
		return
	}
	// Ignore all messages created by the bot itself as well as DMs
	if m.Author.ID == s.State.User.ID || m.GuildID == "" {
		return
	}

	parsedCommand := strings.Split(m.Content, " ")

	switch parsedCommand[0] {
	case "~help":
		go Handle_help(s, m)
	// management commands
	case "~uptime":
		go Handle_uptime(s, m, start)
	case "~shutdown":
		go Handle_shutdown(s, m)
	case "~invite":
		go Handle_invite(s, m)
	case "~nick":
		go Handle_nickname(s, m, parsedCommand)
	case "~kick":
		go Handle_kick(s, m, parsedCommand)
	case "~ban":
		go Handle_ban(s, m, parsedCommand)
	// dbd commands
	case "~perk":
		go Handle_perk(s, m, parsedCommand)
	case "~shrine":
		go Handle_shrine(s, m)
	case "~autoshrine":
		go Handle_autoshrine(s, m, parsedCommand)
	// lookup commands
	case "~define":
		go Handle_define(s, m, parsedCommand)
	case "~google":
		go Handle_google(s, m, parsedCommand)
	case "~image":
		go Handle_image(s, m, parsedCommand)
	}
}

/**
Used to handle scrolling through images given from ~image,
but can and may be used to handle other reactions
*/
func messageReactionAdd(s *discordgo.Session, m *discordgo.MessageReactionAdd) {
	// Ignore all messages created by the bot itself as well as DMs
	if m.UserID == s.State.User.ID {
		return
	}
	for i, set := range globalImageSet {
		if set.MessageID == m.MessageID {
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
