package main

import (
	"fmt"
	"math/rand"
	"os"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/bwmarrin/discordgo"
)

var response string
var timeBetweenCommands time.Duration = 2 * time.Second
var testingChannel string = "739852388264968243"

/**
Test anything that uses calculation outside of the discordgo commands.
Those commands don't need to be tested since they are verified to work
at github.com/bwmarrin/discordgo
**/
func TestMessageResponse(t *testing.T) {
	// main bot
	go runBot(os.Getenv("BOT_TOKEN"))
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
		dg.ChannelMessageSend(testingChannel, "~uptime")
		time.Sleep(timeBetweenCommands) // allow response to populate
		regex := regexp.MustCompile(`^:robot: Uptime: \d\.\ds$`)
		if !regex.MatchString(response) {
			t.Logf("Failed to respond correctly to ~uptime; Response was `" + response + "`")
			t.Fail()
		}
	})

	t.Run("Generates invitation", func(t *testing.T) {
		dg.ChannelMessageSend(testingChannel, "~invite")
		time.Sleep(timeBetweenCommands) // allow response to populate
		if strings.HasPrefix(response, ":mailbox_with_mail: Here's your invitation! https://discord.gg/") == false {
			t.Logf("Failed to generate invitation")
			t.Fail()
		}
	})

	t.Run("Permissions work appropriately", func(t *testing.T) {
		randNum := rand.Intn(10000)
		dg.ChannelMessageSend(testingChannel, fmt.Sprintf("~nick <@!700962207785156668> test nickname %d", randNum))
		time.Sleep(timeBetweenCommands) // allow response to populate
		if response != "Done!" {
			t.Logf("Failed to change valid nickname")
			t.Fail()
		}
	})

	t.Run("Incorrect usages are reported", func(t *testing.T) {
		dg.ChannelMessageSend(testingChannel, "~nick <@!700962207785156668>")
		time.Sleep(timeBetweenCommands) // allow response to populate
		if response != "Usage: `~nick @<user> <new name>`" {
			t.Logf("Should have reported incorrect usage, but didn't")
			t.Fail()
		}

		dg.ChannelMessageSend(testingChannel, "~kick user")
		time.Sleep(timeBetweenCommands) // allow response to populate
		if response != "Usage: `~kick @<user> (reason: optional)`" {
			t.Logf("Should have reported incorrect usage, but didn't")
			t.Fail()
		}

		dg.ChannelMessageSend(testingChannel, "~kick")
		time.Sleep(timeBetweenCommands) // allow response to populate
		if response != "Usage: `~kick @<user> (reason: optional)`" {
			t.Logf("Should have reported incorrect usage, but didn't")
			t.Fail()
		}

		dg.ChannelMessageSend(testingChannel, "~ban user")
		time.Sleep(timeBetweenCommands) // allow response to populate
		if response != "Usage: `~ban @<user> (reason: optional)`" {
			t.Logf("Should have reported incorrect usage, but didn't")
			t.Fail()
		}

		dg.ChannelMessageSend(testingChannel, "~ban")
		time.Sleep(timeBetweenCommands) // allow response to populate
		if response != "Usage: `~ban @<user> (reason: optional)`" {
			t.Logf("Should have reported incorrect usage, but didn't")
			t.Fail()
		}

		dg.ChannelMessageSend(testingChannel, "~profile")
		time.Sleep(timeBetweenCommands) // allow response to populate
		if response != "Usage: `~profile @user`" {
			t.Logf("Should have reported incorrect usage, but didn't")
			t.Fail()
		}

		dg.ChannelMessageSend(testingChannel, "~profile user")
		time.Sleep(timeBetweenCommands) // allow response to populate
		if response != "Usage: `~profile @user`" {
			t.Logf("Should have reported incorrect usage, but didn't")
			t.Fail()
		}

		dg.ChannelMessageSend(testingChannel, "~cp 3")
		time.Sleep(timeBetweenCommands) // allow response to populate
		if response != "Usage: `~cp <number <= 100> <#channel>`" {
			t.Logf("Should have reported incorrect usage, but didn't")
			t.Fail()
		}

		dg.ChannelMessageSend(testingChannel, "~cp 3 fakechannel")
		time.Sleep(timeBetweenCommands) // allow response to populate
		if response != "Usage: `~cp <number <= 100> <#channel>`" {
			t.Logf("Should have reported incorrect usage, but didn't")
			t.Fail()
		}

		dg.ChannelMessageSend(testingChannel, "~mv 3")
		time.Sleep(timeBetweenCommands) // allow response to populate
		if response != "Usage: `~mv <number <= 100> <#channel>`" {
			t.Logf("Should have reported incorrect usage, but didn't")
			t.Fail()
		}

		dg.ChannelMessageSend(testingChannel, "~mv 3 fakechannel")
		time.Sleep(timeBetweenCommands) // allow response to populate
		if response != "Usage: `~mv <number <= 100> <#channel>`" {
			t.Logf("Should have reported incorrect usage, but didn't")
			t.Fail()
		}
	})

	// -- teardown
	time.Sleep(2 * time.Second)
	dg.ChannelMessageSend(testingChannel, "~shutdown")
	dg.Close()
}

func updateResponse(s *discordgo.Session, m *discordgo.MessageCreate) {
	if m.Author.ID == s.State.User.ID {
		return
	}
	response = m.Content
}
