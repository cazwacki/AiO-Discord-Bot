package main

import (
	"fmt"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
)

// validates permissions before moving forward with executing a command
func user_has_valid_permissions(s *discordgo.Session, m *discordgo.MessageCreate, permission int) bool {
	perms, err := s.UserChannelPermissions(m.Author.ID, m.ChannelID)
	if err != nil {
		fmt.Println(err)
		s.ChannelMessageSend(m.ChannelID, "Error occurred while validating your permissions; Restarting...")
		os.Exit(1)
	}
	if perms|permission == perms || perms|discordgo.PermissionAdministrator == perms {
		return true
	} else {
		return false
	}
}

// helper function for ban / kick commands
func dmUser(s *discordgo.Session, m *discordgo.MessageCreate, userID string, message string) {
	channel, err := s.UserChannelCreate(userID)
	if err != nil {
		s.ChannelMessageSend(m.ChannelID, err.Error())
		fmt.Println(err)
	} else {
		s.ChannelMessageSend(channel.ID, message)
	}
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
			s.ChannelMessageSend(m.ChannelID, err.Error())
			fmt.Println(err)
		}
	} else {
		s.ChannelMessageSend(m.ChannelID, "Usage: `~nick @<user> <new name>`")
		fmt.Println(command[1])
	}
}

// helper function for Handle_kick
func attempt_kick(s *discordgo.Session, m *discordgo.MessageCreate, command []string) {
	regex := regexp.MustCompile(`^\<\@\![0-9]+\>$`)
	if len(command) >= 2 {
		if regex.MatchString(command[1]) {
			userID := strings.TrimSuffix(command[1], ">")
			userID = strings.TrimPrefix(userID, "<@!")
			if len(command) > 2 {
				// dm user why they were kicked
				reason := strings.Join(command[2:], " ")
				dmUser(s, m, userID, "You have been kicked by "+m.Author.Username+" for the following reason: '"+reason+"'.")
				// kick with reason
				s.ChannelMessageSend(m.ChannelID, ":wave: Kicked "+command[1]+" for the following reason: '"+reason+"'.")
				s.GuildMemberDeleteWithReason(m.GuildID, userID, reason)
			} else {
				// kick without reason
				dmUser(s, m, userID, "You have been kicked by "+m.Author.Username+".")
				s.ChannelMessageSend(m.ChannelID, ":wave: Kicked "+command[1]+".")
				s.GuildMemberDelete(m.GuildID, userID)
			}
		} else {
			s.ChannelMessageSend(m.ChannelID, "Usage: `~kick @<user> (reason: optional)`")
			fmt.Println(command[1])
		}
	} else {
		s.ChannelMessageSend(m.ChannelID, "Usage: `~kick @<user> (reason: optional)`")
	}
}

func attempt_ban(s *discordgo.Session, m *discordgo.MessageCreate, command []string) {
	regex := regexp.MustCompile(`^\<\@\![0-9]+\>$`)
	if len(command) >= 2 {
		if regex.MatchString(command[1]) {
			userID := strings.TrimSuffix(command[1], ">")
			userID = strings.TrimPrefix(userID, "<@!")
			if len(command) > 2 {
				// dm user why they were banned
				reason := strings.Join(command[2:], " ")
				dmUser(s, m, userID, "You have been banned by "+m.Author.Username+" because: '"+reason+"'.")
				// ban with reason
				s.ChannelMessageSend(m.ChannelID, ":hammer: Banned "+command[1]+" for the following reason: '"+reason+"'.")
				s.GuildBanCreateWithReason(m.GuildID, userID, reason, 0)
			} else {
				// ban without reason
				dmUser(s, m, userID, "You have been banned by "+m.Author.Username+".")
				s.ChannelMessageSend(m.ChannelID, ":hammer: Banned "+command[1]+".")
				s.GuildBanCreate(m.GuildID, userID, 0)
			}
		} else {
			s.ChannelMessageSend(m.ChannelID, "Usage: `~ban @<user> (reason: optional)`")
			fmt.Println(command[1])
		}
	} else {
		s.ChannelMessageSend(m.ChannelID, "Usage: `~ban @<user> (reason: optional)`")
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
	if user_has_valid_permissions(s, m, discordgo.PermissionCreateInstantInvite) {
		var invite discordgo.Invite
		invite.Temporary = true
		invite.MaxAge = 21600 // 6 hours
		invite.MaxUses = 0    // infinite uses
		invite_result, err := s.ChannelInviteCreate(m.ChannelID, invite)
		if err != nil {
			s.ChannelMessageSend(m.ChannelID, "Error creating invite. Try again in a moment.")
			fmt.Println(err)
		} else {
			s.ChannelMessageSend(m.ChannelID, ":mailbox_with_mail: Here's your invitation! https://discord.gg/"+invite_result.Code)
		}
	} else {
		s.ChannelMessageSend(m.ChannelID, "Sorry, you aren't allowed to create an instant invite.")
	}
}

func Handle_nickname(s *discordgo.Session, m *discordgo.MessageCreate, command []string) {
	if user_has_valid_permissions(s, m, discordgo.PermissionChangeNickname) && strings.Contains(command[1], m.Author.ID) {
		// see if user is trying to self-nickname
		attempt_rename(s, m, command)
	} else if user_has_valid_permissions(s, m, discordgo.PermissionManageNicknames) {
		// validate caller has permission to nickname other users
		attempt_rename(s, m, command)
	} else {
		s.ChannelMessageSend(m.ChannelID, "Sorry, you aren't allowed to change nicknames.")
	}
}

func Handle_kick(s *discordgo.Session, m *discordgo.MessageCreate, command []string) {
	if user_has_valid_permissions(s, m, discordgo.PermissionKickMembers) {
		// validate caller has permission to kick other users
		attempt_kick(s, m, command)
	} else {
		s.ChannelMessageSend(m.ChannelID, "Sorry, you aren't allowed to kick users.")
	}
}

func Handle_ban(s *discordgo.Session, m *discordgo.MessageCreate, command []string) {
	if user_has_valid_permissions(s, m, discordgo.PermissionBanMembers) {
		// validate caller has permission to kick other users
		attempt_ban(s, m, command)
	} else {
		s.ChannelMessageSend(m.ChannelID, "Sorry, you aren't allowed to ban users.")
	}
}
