package main

import (
	"fmt"
	"os"
	"regexp"
	"testing"
	"time"

	"github.com/bwmarrin/discordgo"
)

var response string

func TestMessageResponse(t *testing.T) {
	// main bot
	go Run_bot(os.Getenv("BOT_TOKEN"))
	time.Sleep(2 * time.Second) // give bot time to start

	// test bot
	dg, err := discordgo.New("Bot " + os.Getenv("TEST_BOT_TOKEN"))
	if err != nil {
		fmt.Println("Error creating test bot's discord session")
		return
	}
	dg.AddHandler(updateResponse)
	err = dg.Open()
	if err != nil {
		fmt.Println("Error opening test bot's connection,", err)
		return
	}

	// -- tests
	t.Run("Responds to ~uptime", func(t *testing.T) {
		dg.ChannelMessageSend("739852388264968243", "~uptime")
		time.Sleep(200 * time.Millisecond) // allow response to populate
		regex := regexp.MustCompile(`Uptime: \d\.\d{7}s`)
		if !regex.MatchString(response) {
			t.Logf("Failed to respond correctly to ~uptime; Response was `" + response + "`")
			t.Fail()
		}
	})

	// -- teardown
	time.Sleep(time.Second / 2)
	dg.ChannelMessageSend("739852388264968243", "~shutdown")
	dg.Close()
}

func updateResponse(s *discordgo.Session, m *discordgo.MessageCreate) {
	if m.Author.ID == s.State.User.ID {
		return
	}
	response = m.Content
}
