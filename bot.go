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

/**
Handler function when the discord session detects a message is created in
a channel that the bot has access to.
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
	// management commands
	case "~uptime":
		Handle_uptime(s, m, start)
	case "~shutdown":
		Handle_shutdown(s, m)
	case "~invite":
		Handle_invite(s, m)
	case "~nick":
		Handle_nickname(s, m, parsedCommand)
	case "~kick":
		Handle_kick(s, m, parsedCommand)
	case "~ban":
		Handle_ban(s, m, parsedCommand)
	// dbd commands
	case "~perk":
		Handle_perk(s, m, parsedCommand)
	case "~shrine":
		Handle_shrine(s, m)
	case "~autoshrine":
		Handle_autoshrine(s, m, parsedCommand)
		// lookup commands
		/*
			case "~define":
				Handle_define(s, m, parsedCommand)
			case "~google":
				Handle_google(s, m, parsedCommand)
			case "~image":
				Handle_image(s, m, parsedCommand)
			case "~help":
				Handle_help(s, m)
		*/
	}
}
