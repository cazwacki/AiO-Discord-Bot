package main

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/bwmarrin/discordgo"
)

var start time.Time
var prod_mode bool

func Run_bot(token string) {
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

	// Wait here until CTRL-C or other term signal is received.
	fmt.Println("Bot is now running.  Press CTRL-C to exit.")
	sc := make(chan os.Signal, 1)
	signal.Notify(sc, syscall.SIGINT, syscall.SIGTERM, os.Interrupt, os.Kill)
	<-sc

	// Cleanly close down the Discord session.
	dg.Close()
}

func messageCreate(s *discordgo.Session, m *discordgo.MessageCreate) {
	// Ignore my testing channel
	if prod_mode && m.ChannelID == "739852388264968243" {
		return
	}
	// Ignore all messages created by the bot itself
	if m.Author.ID == s.State.User.ID {
		return
	}
	// If the message is "ping" reply with "Pong!"
	if m.Content == "~uptime" {
		s.ChannelMessageSend(m.ChannelID, "Uptime: "+time.Since(start).Truncate(time.Second/10).String())
	}

	if m.Content == "~shutdown" {
		s.ChannelMessageSend(m.ChannelID, "Shutting Down.")
		s.Close()
		os.Exit(0)
	}
}
