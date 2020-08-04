package main

import (
	"fmt"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
)

func userHasValidPermissions(s *discordgo.Session, m *discordgo.MessageCreate, permission int) bool {
	perms, err := s.UserChannelPermissions(m.Author.ID, m.ChannelID)
	if err != nil {
		fmt.Println(err)
		s.ChannelMessageSend(m.ChannelID, "Error occurred while validating your permissions; Restarting...")
		os.Exit(1)
	}
	if perms|permission == perms {
		return true
	} else {
		return false
	}
}

func Handle_uptime(s *discordgo.Session, m *discordgo.MessageCreate, start time.Time) {
	s.ChannelMessageSend(m.ChannelID, "Uptime: "+time.Since(start).Truncate(time.Second/10).String())
}

func Handle_shutdown(s *discordgo.Session, m *discordgo.MessageCreate) {
	s.ChannelMessageSend(m.ChannelID, "Shutting Down.")
	s.Close()
	os.Exit(0)
}

func Handle_invite(s *discordgo.Session, m *discordgo.MessageCreate) {

}

// helper function for Handle_nickname
func attempt_rename(s *discordgo.Session, m *discordgo.MessageCreate, command []string) {
	regex := regexp.MustCompile(`^\<\@\![0-9]+\>$`)
	if regex.MatchString(command[1]) && len(command) > 2 {
		userID := strings.TrimSuffix(command[1], ">")
		userID = strings.TrimPrefix(userID, "<@!")
		err := s.GuildMemberNickname(m.GuildID, userID, strings.Join(command[2:], " "))
		if err == nil {
			s.ChannelMessageSend(m.ChannelID, "Done!")
		} else {
			s.ChannelMessageSend(m.ChannelID, "Error occurred. Check logs for more info")
			fmt.Println(err)
		}
	} else {
		s.ChannelMessageSend(m.ChannelID, "Usage: `~nick @<user> <new name>`")
		fmt.Println(command[1])
	}
}

func Handle_nickname(s *discordgo.Session, m *discordgo.MessageCreate, command []string) {
	if userHasValidPermissions(s, m, discordgo.PermissionChangeNickname) && strings.Contains(command[1], m.Author.ID) {
		// see if user is trying to self-nickname
		attempt_rename(s, m, command)

	} else if userHasValidPermissions(s, m, discordgo.PermissionManageNicknames) || userHasValidPermissions(s, m, discordgo.PermissionAdministrator) {
		// validate caller has permission to nickname other users
		attempt_rename(s, m, command)

	} else {
		s.ChannelMessageSend(m.ChannelID, "Sorry, you aren't allowed to change nicknames.")
	}
}

func Handle_kick(s *discordgo.Session, m *discordgo.MessageCreate, command []string) {
	regex := regexp.MustCompile(`^\<\@\![0-9]+\>$`)
	if regex.MatchString(command[1]) {

	} else {
		s.ChannelMessageSend(m.ChannelID, "Usage: `~kick @<user> (reason: optional)`")
	}
}

func Handle_ban(s *discordgo.Session, m *discordgo.MessageCreate, command []string) {
	regex := regexp.MustCompile(`^\<\@\![0-9]+\>$`)
	if regex.MatchString(command[1]) {

	} else {
		s.ChannelMessageSend(m.ChannelID, "Usage: `~ban @<user> (reason: optional)`")
	}
}
