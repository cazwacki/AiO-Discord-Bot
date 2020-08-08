package main

import (
	"fmt"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
)

/**
Used to validate a user's permissions before moving forward with a command. Prevents command abuse.
If the user has administrator permissions, just automatically allow them to perform any bot command.
**/
func userHasValidPermissions(s *discordgo.Session, m *discordgo.MessageCreate, permission int) bool {
	perms, err := s.UserChannelPermissions(m.Author.ID, m.ChannelID)
	if err != nil {
		fmt.Println(err)
		s.ChannelMessageSend(m.ChannelID, "Error occurred while validating your permissions; Restarting...")
		os.Exit(1)
	}
	if perms|permission == perms || perms|discordgo.PermissionAdministrator == perms {
		return true
	}
	return false
}

/**
Given a userID, generates a DM if one does not already exist with the user and sends the specified
message to them.
**/
func dmUser(s *discordgo.Session, m *discordgo.MessageCreate, userID string, message string) {
	channel, err := s.UserChannelCreate(userID)
	if err != nil {
		s.ChannelMessageSend(m.ChannelID, err.Error())
		fmt.Println(err)
	} else {
		s.ChannelMessageSend(channel.ID, message)
	}
}

/**
A helper function for Handle_nick. Ensures the user targeted a user using @; if they did,
attempt to rename the specified user.
**/
func attemptRename(s *discordgo.Session, m *discordgo.MessageCreate, command []string) {
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

/**
A helper function for Handle_kick. Ensures the user targeted a user using @; if they did,
attempt to kick the specified user.
**/
func attemptKick(s *discordgo.Session, m *discordgo.MessageCreate, command []string) {
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

/**
A helper function for Handle_ban. Ensures the user targeted a user using @; if they did,
attempt to ban the specified user.
**/
func attemptBan(s *discordgo.Session, m *discordgo.MessageCreate, command []string) {
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

/**
Outputs the bot's current uptime.
**/
func handleUptime(s *discordgo.Session, m *discordgo.MessageCreate, start time.Time) {
	s.ChannelMessageSend(m.ChannelID, ":robot: Uptime: "+time.Since(start).Truncate(time.Second/10).String())
}

/**
Forces the bot to exit with code 0. Note that in Heroku the bot will restart automatically.
**/
func handleShutdown(s *discordgo.Session, m *discordgo.MessageCreate) {
	s.ChannelMessageSend(m.ChannelID, "Shutting Down.")
	s.Close()
	os.Exit(0)
}

/**
Generates an invite code to the channel in which ~invite was invoked if the user has the
permission to create instant invites.
**/
func handleInvite(s *discordgo.Session, m *discordgo.MessageCreate) {
	if userHasValidPermissions(s, m, discordgo.PermissionCreateInstantInvite) {
		var invite discordgo.Invite
		invite.Temporary = false
		invite.MaxAge = 21600 // 6 hours
		invite.MaxUses = 0    // infinite uses
		inviteResult, err := s.ChannelInviteCreate(m.ChannelID, invite)
		if err != nil {
			s.ChannelMessageSend(m.ChannelID, "Error creating invite. Try again in a moment.")
			fmt.Println(err)
		} else {
			s.ChannelMessageSend(m.ChannelID, ":mailbox_with_mail: Here's your invitation! https://discord.gg/"+inviteResult.Code)
		}
	} else {
		s.ChannelMessageSend(m.ChannelID, "Sorry, you aren't allowed to create an instant invite.")
	}
}

/**
Nicknames the user if they target themselves, or nicknames a target user if the user who invoked
~nick has the permission to change nicknames.
**/
func handleNickname(s *discordgo.Session, m *discordgo.MessageCreate, command []string) {
	if userHasValidPermissions(s, m, discordgo.PermissionChangeNickname) && strings.Contains(command[1], m.Author.ID) {
		// see if user is trying to self-nickname
		attemptRename(s, m, command)
	} else if userHasValidPermissions(s, m, discordgo.PermissionManageNicknames) {
		// validate caller has permission to nickname other users
		attemptRename(s, m, command)
	} else {
		s.ChannelMessageSend(m.ChannelID, "Sorry, you aren't allowed to change nicknames.")
	}
}

/**
Kicks a user from the server if the invoking user has the permission to kick users.
**/
func handleKick(s *discordgo.Session, m *discordgo.MessageCreate, command []string) {
	if userHasValidPermissions(s, m, discordgo.PermissionKickMembers) {
		// validate caller has permission to kick other users
		attemptKick(s, m, command)
	} else {
		s.ChannelMessageSend(m.ChannelID, "Sorry, you aren't allowed to kick users.")
	}
}

/**
Bans a user from the server if the invoking user has the permission to ban users.
**/
func handleBan(s *discordgo.Session, m *discordgo.MessageCreate, command []string) {
	if userHasValidPermissions(s, m, discordgo.PermissionBanMembers) {
		// validate caller has permission to kick other users
		attemptBan(s, m, command)
	} else {
		s.ChannelMessageSend(m.ChannelID, "Sorry, you aren't allowed to ban users.")
	}
}
