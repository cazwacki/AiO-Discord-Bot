package main

import (
	"fmt"
	"math/rand"
	"os"
	"regexp"
	"testing"
	"time"

	"github.com/bwmarrin/discordgo"
)

var response string

/**
Test anything that uses calculation outside of the discordgo commands.
Those commands don't need to be tested since they are verified to work
at github.com/bwmarrin/discordgo
**/
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
	t.Run("Responds correctly to ~uptime", func(t *testing.T) {
		dg.ChannelMessageSend("739852388264968243", "~uptime")
		time.Sleep(time.Second) // allow response to populate
		regex := regexp.MustCompile(`^Uptime: \d\.\ds$`)
		if !regex.MatchString(response) {
			t.Logf("Failed to respond correctly to ~uptime; Response was `" + response + "`")
			t.Fail()
		}
	})

	t.Run("Permissions work appropriately", func(t *testing.T) {
		randNum := rand.Intn(10000)
		dg.ChannelMessageSend("739852388264968243", fmt.Sprintf("~nick <@!700962207785156668> test nickname %d", randNum))
		time.Sleep(time.Second) // allow response to populate
		if response != "Done!" {
			t.Logf("Failed to change valid nickname")
			t.Fail()
		}
	})

	t.Run("Incorrect usages are reported", func(t *testing.T) {
		dg.ChannelMessageSend("739852388264968243", "~nick <@!700962207785156668>")
		time.Sleep(time.Second) // allow response to populate
		if response != "Usage: `~nick @<user> <new name>`" {
			t.Logf("Should have reported incorrect usage, but didn't")
			t.Fail()
		}
	})

	// -- teardown
	time.Sleep(2 * time.Second)
	dg.ChannelMessageSend("739852388264968243", "~shutdown")
	dg.Close()
}

func updateResponse(s *discordgo.Session, m *discordgo.MessageCreate) {
	if m.Author.ID == s.State.User.ID {
		return
	}
	response = m.Content
}