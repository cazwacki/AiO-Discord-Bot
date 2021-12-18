package main

import (
	"encoding/base64"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
)

/**
Used to validate a user's permissions before moving forward with a command. Prevents command abuse.
If the user has administrator permissions, just automatically allow them to perform any bot command.
**/
func userHasValidPermissions(s *discordgo.Session, m *discordgo.MessageCreate, permission int64) bool {
	perms, err := s.UserChannelPermissions(m.Author.ID, m.ChannelID)
	if err != nil {
		attemptSendMsg(s, m, "Error occurred while validating your permissions.")
		logError("Failed to acquire user permissions! " + err.Error())
		return false
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
func dmUser(s *discordgo.Session, userID string, message string) {
	channel, err := s.UserChannelCreate(userID)
	if err != nil {
		logError("Failed to create DM with user. " + err.Error())
		return
	}
	_, err = s.ChannelMessageSend(channel.ID, message)
	if err != nil {
		logError("Failed to send message! " + err.Error())
		return
	}
	logSuccess("Sent DM to user")
}

/**
A helper function for Handle_nick. Ensures the user targeted a user using @; if they did,
attempt to rename the specified user.
**/
func attemptRename(s *discordgo.Session, m *discordgo.MessageCreate, command []string) {
	logInfo(strings.Join(command, " "))
	regex := regexp.MustCompile(`^\<\@\!?[0-9]+\>$`)
	if regex.MatchString(command[1]) && len(command) > 2 {
		userID := stripUserID(command[1])
		err := s.GuildMemberNickname(m.GuildID, userID, strings.Join(command[2:], " "))
		if err == nil {
			attemptSendMsg(s, m, "Done!")
			logSuccess("Successfully renamed user")
		} else {
			attemptSendMsg(s, m, fmt.Sprintf("Failed to set nickname.\n```%s```", err.Error()))
			logError("Failed to set nickname! " + err.Error())
			return
		}
		return
	}

	attemptSendMsg(s, m, "Usage: `~nick @<user> <new name>`")
}

/**
A helper function for Handle_kick. Ensures the user targeted a user using @; if they did,
attempt to kick the specified user.
**/
func attemptKick(s *discordgo.Session, m *discordgo.MessageCreate, command []string) {
	logInfo(strings.Join(command, " "))
	regex := regexp.MustCompile(`^\<\@\!?[0-9]+\>$`)
	if len(command) >= 2 {
		if regex.MatchString(command[1]) {
			userID := stripUserID(command[1])
			if len(command) > 2 {
				reason := strings.Join(command[2:], " ")
				// dm user why they were kicked
				guild, err := s.Guild(m.GuildID)
				if err != nil {
					logError("Unable to load guild! " + err.Error())
				}
				guildName := "error: could not retrieve"
				if guild != nil {
					guildName = guild.Name
				}
				dmUser(s, userID, fmt.Sprintf("You have been kicked from **%s** by %s#%s because: %s\n", guildName, m.Author.Username, m.Author.Discriminator, reason))

				// kick with reason
				err = s.GuildMemberDeleteWithReason(m.GuildID, userID, reason)
				if err != nil {
					attemptSendMsg(s, m, "Failed to kick the user.")
					logError("Failed to kick user! " + err.Error())
					return
				}

				attemptSendMsg(s, m, fmt.Sprintf(":wave: Kicked %s for the following reason: '%s'.", command[1], reason))
				logSuccess("Kicked user with reason")
			} else {
				// dm user they were kicked
				guild, err := s.Guild(m.GuildID)
				if err != nil {
					logError("Unable to load guild! " + err.Error())
				}
				guildName := "error: could not retrieve"
				if guild != nil {
					guildName = guild.Name
				}
				dmUser(s, userID, fmt.Sprintf("You have been kicked from **%s** by %s#%s.\n", guildName, m.Author.Username, m.Author.Discriminator))
				// kick without reason
				err = s.GuildMemberDelete(m.GuildID, userID)
				if err != nil {
					attemptSendMsg(s, m, "Failed to kick the user.")
					logError("Failed to kick user! " + err.Error())
					return
				}
				attemptSendMsg(s, m, fmt.Sprintf(":wave: Kicked %s.", command[1]))
				logSuccess("Kicked user")
			}
			return
		}
	}
	attemptSendMsg(s, m, "Usage: `~kick @<user> (reason: optional)`")
}

/**
A helper function for Handle_ban. Ensures the user targeted a user using @; if they did,
attempt to ban the specified user.
**/
func attemptBan(s *discordgo.Session, m *discordgo.MessageCreate, command []string) {
	logInfo(strings.Join(command, " "))
	regex := regexp.MustCompile(`^\<\@\!?[0-9]+\>$`)
	if len(command) >= 2 {
		if regex.MatchString(command[1]) {
			userID := stripUserID(command[1])
			if len(command) > 2 {
				reason := strings.Join(command[2:], " ")

				// dm user why they were banned
				guild, err := s.Guild(m.GuildID)
				if err != nil {
					logError("Unable to load guild! " + err.Error())
				}
				guildName := "error: could not retrieve"
				if guild != nil {
					guildName = guild.Name
				}
				dmUser(s, userID, fmt.Sprintf("You have been banned from **%s** by %s#%s because: %s\n", guildName, m.Author.Username, m.Author.Discriminator, reason))

				// ban with reason
				err = s.GuildBanCreateWithReason(m.GuildID, userID, reason, 0)
				if err != nil {
					attemptSendMsg(s, m, "Failed to ban the user.")
					logError("Failed to ban user! " + err.Error())
					if err != nil {
						logWarning("Failed to send failure message! " + err.Error())
					}
					return
				}

				attemptSendMsg(s, m, fmt.Sprintf(":hammer: Banned %s for the following reason: '%s'.", command[1], reason))
				logSuccess("Banned user with reason without issue")
			} else {
				// ban without reason
				err := s.GuildBanCreate(m.GuildID, userID, 0)
				if err != nil {
					attemptSendMsg(s, m, "Failed to ban the user.")
					logError("Failed to ban user! " + err.Error())
					return
				}
				// dm user they were banned
				guild, err := s.Guild(m.GuildID)
				if err != nil {
					logError("Unable to load guild! " + err.Error())
				}
				guildName := "error: could not retrieve"
				if guild != nil {
					guildName = guild.Name
				}
				dmUser(s, userID, fmt.Sprintf("You have been banned from **%s** by %s#%s.\n", guildName, m.Author.Username, m.Author.Discriminator))

				attemptSendMsg(s, m, fmt.Sprintf(":hammer: Banned %s.", command[1]))
				logSuccess("Banned user with reason without issue")
			}
			return
		}
	}
	attemptSendMsg(s, m, "Usage: `~ban @<user> (reason: optional)`")
}

/**
Attempts to purge the last <number> messages, then removes the purge command.
*/
func attemptPurge(s *discordgo.Session, m *discordgo.MessageCreate, command []string) {
	logInfo(strings.Join(command, " "))
	if len(command) == 2 {
		messageCount, err := strconv.Atoi(command[1])
		if err != nil {
			attemptSendMsg(s, m, "Usage: `~purge <number> (optional: @user)`")
			return
		}
		if messageCount < 1 {
			attemptSendMsg(s, m, ":frowning: Sorry, you must purge at least 1 message. Try again.")
			logWarning("User attempted to purge < 1 message.")
			return
		}
		for messageCount > 0 {
			messagesToPurge := 0
			// can only purge 100 messages per invocation
			if messageCount > 100 {
				messagesToPurge = 100
			} else {
				messagesToPurge = messageCount
			}

			// get the last (messagesToPurge) messages from the channel
			messages, err := s.ChannelMessages(m.ChannelID, messagesToPurge, m.ID, "", "")
			if err != nil {
				attemptSendMsg(s, m, ":frowning: I couldn't pull messages from the channel. Try again.")
				logError("Failed to pull messages from channel! " + err.Error())
				return
			}

			// stop purging if there is nothing left to purge
			if len(messages) < messagesToPurge {
				messageCount = 0
			}

			// get the message IDs
			var messageIDs []string
			for _, message := range messages {
				messageIDs = append(messageIDs, message.ID)
			}

			// delete all the marked messages
			err = s.ChannelMessagesBulkDelete(m.ChannelID, messageIDs)
			if err != nil {
				logWarning("Failed to bulk delete messages! Attempting to continue... " + err.Error())
			}
			messageCount -= messagesToPurge
		}
		time.Sleep(time.Second)
		err = s.ChannelMessageDelete(m.ChannelID, m.ID)
		if err != nil {
			logError("Failed to delete invoked command! " + err.Error())
			return
		}
		logSuccess("Purged all messages, including command invoked")
	} else {
		attemptSendMsg(s, m, "Usage: `~purge <number>`")
	}
}

/**
Attempts to copy over the last <number> messages to the given channel, then outputs its success
*/
func attemptCopy(s *discordgo.Session, m *discordgo.MessageCreate, command []string, preserveMessages bool) {
	logInfo(strings.Join(command, " "))
	var commandInvoked string
	if preserveMessages {
		commandInvoked = "cp"
	} else {
		commandInvoked = "mv"
	}

	if len(command) != 3 {
		attemptSendMsg(s, m, fmt.Sprintf("Usage: `~%s <number <= 100> <#channel>`", commandInvoked))
		return
	}

	messageCount, err := strconv.Atoi(command[1])
	if err != nil {
		attemptSendMsg(s, m, "Failed to read the message count.")
		return
	}

	// verify correctly invoking channel
	if !strings.HasPrefix(command[2], "<#") || !strings.HasSuffix(command[2], ">") {
		attemptSendMsg(s, m, fmt.Sprintf("Usage: `~%s <number <= 100> <#channel>`", commandInvoked))
		return
	}
	channel := strings.ReplaceAll(command[2], "<#", "")
	channel = strings.ReplaceAll(channel, ">", "")

	// retrieve messages from current invoked channel
	messages, err := s.ChannelMessages(m.ChannelID, messageCount, m.ID, "", "")
	if err != nil {
		attemptSendMsg(s, m, "Ran into an error retrieving messages. :slight_frown:")
		return
	}

	// construct an embed for each message
	for index := range messages {
		var embed discordgo.MessageEmbed
		embed.Type = "rich"
		message := messages[len(messages)-1-index]

		// remove messages if calling mv command
		if !preserveMessages {
			err := s.ChannelMessageDelete(m.ChannelID, message.ID)
			if err != nil {
				logWarning("Failed to delete a message. Attempting to continue... " + err.Error())
			}
		}

		// populating author information in the embed
		var embedAuthor discordgo.MessageEmbedAuthor
		if message.Author != nil {
			member, err := s.GuildMember(m.GuildID, message.Author.ID)
			nickname := ""
			if err == nil {
				nickname = member.Nick
			} else {
				logWarning("Could not find a nickname for the user! " + err.Error())
			}
			embedAuthor.Name = ""
			if nickname != "" {
				embedAuthor.Name += nickname + " ("
			}
			embedAuthor.Name += message.Author.Username + "#" + message.Author.Discriminator
			if nickname != "" {
				embedAuthor.Name += ")"
			}
			embedAuthor.IconURL = message.Author.AvatarURL("")
		}
		embed.Author = &embedAuthor

		// preserve message timestamp
		embed.Timestamp = string(message.Timestamp)
		var contents []*discordgo.MessageEmbedField

		// output message text
		logInfo("Message Content: " + message.Content)
		if message.Content != "" {
			embed.Description = message.Content
		}

		// output attachments
		logInfo(fmt.Sprintf("Attachments: %d\n", len(message.Attachments)))
		if len(message.Attachments) > 0 {
			for _, attachment := range message.Attachments {
				contents = append(contents, createField("Attachment: "+attachment.Filename, attachment.ProxyURL, false))
			}
		}

		// output embed contents (up to 10... jesus christ...)
		logInfo(fmt.Sprintf("Embeds: %d\n", len(message.Embeds)))
		if len(message.Embeds) > 0 {
			for _, embed := range message.Embeds {
				contents = append(contents, createField("Embed Title", embed.Title, false))
				contents = append(contents, createField("Embed Text", embed.Description, false))
				if embed.Image != nil {
					contents = append(contents, createField("Embed Image", embed.Image.ProxyURL, false))
				}
				if embed.Thumbnail != nil {
					contents = append(contents, createField("Embed Thumbnail", embed.Thumbnail.ProxyURL, false))
				}
				if embed.Video != nil {
					contents = append(contents, createField("Embed Video", embed.Video.URL, false))
				}
				if embed.Footer != nil {
					contents = append(contents, createField("Embed Footer", embed.Footer.Text, false))
				}
			}
		}

		// ouput reactions on a message
		if len(message.Reactions) > 0 {
			reactionText := ""
			for index, reactionSet := range message.Reactions {
				reactionText += reactionSet.Emoji.Name + " x" + strconv.Itoa(reactionSet.Count)
				if index < len(message.Reactions)-1 {
					reactionText += ", "
				}
			}
			contents = append(contents, createField("Reactions", reactionText, false))
		}
		embed.Fields = contents

		// send response
		_, err := s.ChannelMessageSendEmbed(channel, &embed)
		if err != nil {
			logError("Failed to send result message! " + err.Error())
			return
		}
	}
	_, err = s.ChannelMessageSend(m.ChannelID, "Copied "+strconv.Itoa(messageCount)+" messages from <#"+m.ChannelID+"> to <#"+channel+">! :smile:")
	if err != nil {
		logError("Failed to send success message! " + err.Error())
		return
	}
	logSuccess("Copied messages and sent success message")
}

/**
Helper function for handleProfile. Attempts to retrieve a user's avatar and return it
in an embed.
*/
func attemptProfile(s *discordgo.Session, m *discordgo.MessageCreate, command []string) {
	logInfo(strings.Join(command, " "))
	if len(command) == 2 {
		regex := regexp.MustCompile(`^\<\@\!?[0-9]+\>$`)
		if regex.MatchString(command[1]) {
			userID := strings.TrimSuffix(command[1], ">")
			userID = strings.TrimPrefix(userID, "<@")
			userID = strings.TrimPrefix(userID, "!") // this means the user has a nickname
			var embed discordgo.MessageEmbed
			embed.Type = "rich"

			// get user
			user, err := s.User(userID)
			if err != nil {
				attemptSendMsg(s, m, "Error retrieving the user. :frowning:")
				logError("Could not retrieve user from session! " + err.Error())
				return
			}

			// get member data from the user
			member, err := s.GuildMember(m.GuildID, userID)
			nickname := ""
			if err == nil {
				nickname = member.Nick
			} else {
				fmt.Println(err)
			}

			// title the embed
			embed.Title = "Profile Picture for "
			if nickname != "" {
				embed.Title += nickname + " ("
			}
			embed.Title += user.Username + "#" + user.Discriminator
			if nickname != "" {
				embed.Title += ")"
			}

			// attach the user's avatar as 512x512 image
			var image discordgo.MessageEmbedImage
			image.URL = user.AvatarURL("512")
			embed.Image = &image

			_, err = s.ChannelMessageSendEmbed(m.ChannelID, &embed)
			if err != nil {
				logError("Failed to send result message! " + err.Error())
				return
			}
			logSuccess("Returned user profile picture")
			return
		}
	}
	attemptSendMsg(s, m, "Usage: `~profile @user`")
}

func attemptAbout(s *discordgo.Session, m *discordgo.MessageCreate, command []string) {
	logInfo(strings.Join(command, " "))
	if len(command) == 2 {
		regex := regexp.MustCompile(`^\<\@\!?[0-9]+\>$`)
		if regex.MatchString(command[1]) {
			userID := stripUserID(command[1])

			logInfo(strings.Join(command, " "))

			member, err := s.GuildMember(m.GuildID, userID)
			if err != nil {
				logError("Could not retrieve user from the session! " + err.Error())
				attemptSendMsg(s, m, "Error retrieving the user. :frowning:")
				return
			}

			var embed discordgo.MessageEmbed
			embed.Type = "rich"

			// title the embed
			embed.Title = "About " + member.User.Username + "#" + member.User.Discriminator

			var contents []*discordgo.MessageEmbedField

			joinDate, err := member.JoinedAt.Parse()
			if err != nil {
				logError("Failed to parse Discord dates! " + err.Error())
				attemptSendMsg(s, m, "Error parsing Discord's dates. :frowning:")
				return
			}

			nickname := "N/A"
			if member.Nick != "" {
				nickname = member.Nick
			}

			contents = append(contents, createField("Server Join Date", joinDate.Format("01/02/2006"), false))
			contents = append(contents, createField("Nickname", nickname, false))

			// get user's roles in readable form
			guildRoles, err := s.GuildRoles(m.GuildID)
			if err != nil {
				logError("Failed to retrieve guild roles! " + err.Error())
				attemptSendMsg(s, m, "Error retrieving the guild's roles. :frowning:")
				return
			}
			var rolesAttached []string

			for _, role := range guildRoles {
				for _, roleID := range member.Roles {
					if role.ID == roleID {
						rolesAttached = append(rolesAttached, role.Name)
					}
				}
			}
			contents = append(contents, createField("Roles", strings.Join(rolesAttached, ", "), false))

			embed.Fields = contents

			// send response
			_, err = s.ChannelMessageSendEmbed(m.ChannelID, &embed)
			if err != nil {
				logError("Couldn't send the message... " + err.Error())
				return
			}
			logSuccess("Returned user information")
			return
		}
	}
	attemptSendMsg(s, m, "Usage: `~about @user`")
}

/**
Outputs the bot's current uptime.
**/
func handleUptime(s *discordgo.Session, m *discordgo.MessageCreate, start []string) {
	logInfo(start[0])
	start_time, err := time.Parse("2006-01-02 15:04:05.999999999 -0700 MST", start[0])
	if err != nil {
		logError("Could not parse start time! " + err.Error())
		attemptSendMsg(s, m, "Error parsing the date... :frowning:")
	}
	attemptSendMsg(s, m, fmt.Sprintf(":robot: Uptime: %s", time.Since(start_time).Truncate(time.Second/10).String()))
	logSuccess("Reported uptime")
}

/**
Toggles "deafened" state of the specified user.
**/
func vcDeaf(s *discordgo.Session, m *discordgo.MessageCreate, command []string) {
	if !userHasValidPermissions(s, m, discordgo.PermissionVoiceDeafenMembers) {
		attemptSendMsg(s, m, "Sorry, you aren't allowed to deafen/undeafen other members.")
		return
	}

	if len(command) != 2 {
		attemptSendMsg(s, m, "Usage: `~vcdeaf @user`")
		return
	}

	// 1. get user ID
	regex := regexp.MustCompile(`^\<\@\!?[0-9]+\>$`)
	if !regex.MatchString(command[1]) {
		attemptSendMsg(s, m, "Usage: `~vcdeaf @user`")
		return
	}

	userID := stripUserID(command[1])

	logInfo(strings.Join(command, " "))

	member, err := s.GuildMember(m.GuildID, userID)
	if err != nil {
		logError("Could not retrieve user from the session! " + err.Error())
		attemptSendMsg(s, m, "Error retrieving the user. :frowning:")
		return
	}

	// 2. toggle deafened state
	err = s.GuildMemberDeafen(m.GuildID, userID, !member.Deaf)
	if err != nil {
		logError("Failed to toggle deafened state of the user! " + err.Error())
		attemptSendMsg(s, m, "Failed to toggle deafened state of the user.")
		return
	}

	attemptSendMsg(s, m, "Toggled 'deafened' state of the user.")
}

/**
Toggles "VC muted" state of the specified user.
**/
func vcMute(s *discordgo.Session, m *discordgo.MessageCreate, command []string) {
	if !userHasValidPermissions(s, m, discordgo.PermissionVoiceMuteMembers) {
		attemptSendMsg(s, m, "Sorry, you aren't allowed to mute/unmute other members.")
		return
	}

	if len(command) != 2 {
		attemptSendMsg(s, m, "Usage: `~vcmute @user`")
		return
	}

	// 1. get user ID
	regex := regexp.MustCompile(`^\<\@\!?[0-9]+\>$`)
	if !regex.MatchString(command[1]) {
		attemptSendMsg(s, m, "Usage: `~vcdeaf @user`")
		return
	}

	userID := stripUserID(command[1])

	logInfo(strings.Join(command, " "))

	member, err := s.GuildMember(m.GuildID, userID)
	if err != nil {
		logError("Could not retrieve user from the session! " + err.Error())
		attemptSendMsg(s, m, "Error retrieving the user. :frowning:")
		return
	}

	// 2. toggled vc muted state
	err = s.GuildMemberMute(m.GuildID, userID, !member.Mute)
	if err != nil {
		logError("Failed to toggle muted state of the user! " + err.Error())
		attemptSendMsg(s, m, "Failed to toggle muted state of the user.")
		return
	}

	attemptSendMsg(s, m, "Toggled 'muted' state of the user from the channel.")
}

/**
Moves the user to the specified voice channel.
**/
func vcMove(s *discordgo.Session, m *discordgo.MessageCreate, command []string) {
	if !userHasValidPermissions(s, m, discordgo.PermissionVoiceDeafenMembers) {
		attemptSendMsg(s, m, "Sorry, you aren't allowed to move other members across voice channels.")
		return
	}

	if len(command) != 3 {
		attemptSendMsg(s, m, "Usage: `~vcmove @user #!<voice channel>`")
		return
	}

	// 1. get user ID
	regex := regexp.MustCompile(`^\<\@\!?[0-9]+\>$`)
	if !regex.MatchString(command[1]) {
		attemptSendMsg(s, m, "Usage: `~vcdeaf @user`")
		return
	}

	userID := stripUserID(command[1])

	logInfo(strings.Join(command, " "))

	// 2. get voice channel
	if !strings.HasPrefix(command[2], "<#") || !strings.HasSuffix(command[2], ">") {
		attemptSendMsg(s, m, "Usage: `~vcmove @user #!<voice channel>`")
		return
	}
	channel := strings.ReplaceAll(command[2], "<#", "")
	channel = strings.ReplaceAll(channel, ">", "")

	// 3. move user to voice channel
	err := s.GuildMemberMove(m.GuildID, userID, &channel)
	if err != nil {
		logError("Failed to move the user to that channel! " + err.Error())
		attemptSendMsg(s, m, "Failed to move the user to that channel. Is it a voice channel?")
		return
	}

	attemptSendMsg(s, m, "Moved the user.")
}

/**
Kicks the specified user from the voice channel they are in, if any.
**/
func vcKick(s *discordgo.Session, m *discordgo.MessageCreate, command []string) {
	if !userHasValidPermissions(s, m, discordgo.PermissionVoiceMuteMembers) {
		attemptSendMsg(s, m, "Sorry, you aren't allowed to mute/unmute other members.")
		return
	}

	if len(command) != 2 {
		attemptSendMsg(s, m, "Usage: `~vckick @user`")
		return
	}

	// 1. get user ID
	regex := regexp.MustCompile(`^\<\@\!?[0-9]+\>$`)
	if !regex.MatchString(command[1]) {
		attemptSendMsg(s, m, "Usage: `~vckick @user`")
		return
	}

	userID := stripUserID(command[1])

	// 2. remove user from voice channel
	err := s.GuildMemberMove(m.GuildID, userID, nil)
	if err != nil {
		logError("Failed to remove the user from that channel! " + err.Error())
		attemptSendMsg(s, m, "Failed to remove the user from that channel. Is it a voice channel?")
		return
	}

	attemptSendMsg(s, m, "Kicked the user from the channel.")
}

/**
Forces the bot to exit with code 0. Note that in Heroku the bot will restart automatically.
**/
func handleShutdown(s *discordgo.Session, m *discordgo.MessageCreate, command []string) {
	logInfo(strings.Join(command, " "))
	if m.Author.ID == "172311520045170688" {
		attemptSendMsg(s, m, "Shutting Down.")
		s.Close()
		os.Exit(0)
	} else {
		attemptSendMsg(s, m, "You dare try and go against the wishes of <@172311520045170688> ..? ")

		time.Sleep(10 * time.Second)
		attemptSendMsg(s, m, "Bruh this gonna be you when sage and his boys get here... I just pinged him so you better be afraid :slight_smile:")

		time.Sleep(2 * time.Second)
		attemptSendMsg(s, m, "https://media4.giphy.com/media/3o6Ztm3eJNDBy4NfiM/giphy.gif")
	}
}

/**
Generates an invite code to the channel in which ~invite was invoked if the user has the
permission to create instant invites.
**/
func handleInvite(s *discordgo.Session, m *discordgo.MessageCreate, command []string) {
	logInfo(strings.Join(command, " "))
	if !userHasValidPermissions(s, m, discordgo.PermissionCreateInstantInvite) {
		attemptSendMsg(s, m, "Sorry, you aren't allowed to create an instant invite.")
		return
	}
	var invite discordgo.Invite
	invite.Temporary = false
	invite.MaxAge = 21600 // 6 hours
	invite.MaxUses = 0    // infinite uses
	inviteResult, err := s.ChannelInviteCreate(m.ChannelID, invite)
	if err != nil {
		attemptSendMsg(s, m, "Error creating invite. Try again in a moment.")
		logError("Failed to generate invite! " + err.Error())
		return
	} else {
		attemptSendMsg(s, m, fmt.Sprintf(":mailbox_with_mail: Here's your invitation! https://discord.gg/%s", inviteResult.Code))
	}
	logSuccess("Generated and sent invite")
}

/**
Nicknames the user if they target themselves, or nicknames a target user if the user who invoked
~nick has the permission to change nicknames.
**/
func handleNickname(s *discordgo.Session, m *discordgo.MessageCreate, command []string) {
	if !(userHasValidPermissions(s, m, discordgo.PermissionChangeNickname) && strings.Contains(command[1], m.Author.ID)) && !(userHasValidPermissions(s, m, discordgo.PermissionManageNicknames)) {
		attemptSendMsg(s, m, "Sorry, you aren't allowed to change nicknames.")
		return
	}
	attemptRename(s, m, command)
}

/**
Kicks a user from the server if the invoking user has the permission to kick users.
**/
func handleKick(s *discordgo.Session, m *discordgo.MessageCreate, command []string) {
	if !userHasValidPermissions(s, m, discordgo.PermissionKickMembers) {
		attemptSendMsg(s, m, "Sorry, you aren't allowed to kick users.")
		return
	}
	attemptKick(s, m, command)
}

/**
Bans a user from the server if the invoking user has the permission to ban users.
**/
func handleBan(s *discordgo.Session, m *discordgo.MessageCreate, command []string) {
	if !userHasValidPermissions(s, m, discordgo.PermissionBanMembers) {
		attemptSendMsg(s, m, "Sorry, you aren't allowed to ban users.")
		return
	}
	attemptBan(s, m, command)
}

/**
Removes the <number> most recent messages from the channel where the command was called.
**/
func handlePurge(s *discordgo.Session, m *discordgo.MessageCreate, command []string) {
	if !userHasValidPermissions(s, m, discordgo.PermissionManageMessages) {
		attemptSendMsg(s, m, "Sorry, you aren't allowed to remove messages.")
		return
	}
	attemptPurge(s, m, command)
}

/**
Copies the <number> most recent messages from the channel where the command was called and
pastes it in the requested channel.
**/
func handleCopy(s *discordgo.Session, m *discordgo.MessageCreate, command []string) {
	if !userHasValidPermissions(s, m, discordgo.PermissionManageMessages) {
		attemptSendMsg(s, m, "Sorry, you aren't allowed to manage messages.")
		return
	}
	attemptCopy(s, m, command, true)
}

/**
Same as above, but purges each message it copies
**/
func handleMove(s *discordgo.Session, m *discordgo.MessageCreate, command []string) {
	if !userHasValidPermissions(s, m, discordgo.PermissionManageMessages) {
		attemptSendMsg(s, m, "Sorry, you aren't allowed to manage messages.")
		return
	}
	attemptCopy(s, m, command, false)
}

/**
Allows user to create, remove, or edit emojis associated with the server.
**/
func emoji(s *discordgo.Session, m *discordgo.MessageCreate, command []string) {
	logInfo(strings.Join(command, " "))
	// validate calling user has permission to manage emojis
	if !userHasValidPermissions(s, m, discordgo.PermissionManageEmojis) {
		attemptSendMsg(s, m, "Sorry, you aren't allowed to manage emojis.")
		return
	}

	if len(command) == 1 {
		// send usage information
		var embed discordgo.MessageEmbed
		embed.Type = "rich"
		embed.Title = "Emoji Commands"
		embed.Description = "Create, delete, or edit your server emojis quickly and easily."

		var contents []*discordgo.MessageEmbedField
		contents = append(contents, createField("emoji create <name> <url>", "Create a new server emoji with the given name using the image from the provided URL.", false))
		contents = append(contents, createField("emoji rename <emoji> <new name>", "Set an existing emoji's name to the name passed in <new name>.", false))
		contents = append(contents, createField("emoji delete <emoji>", "Remove the selected emoji from the server.", false))
		embed.Fields = contents

		_, err := s.ChannelMessageSendEmbed(m.ChannelID, &embed)
		if err != nil {
			logError("Failed to send instructions message embed! " + err.Error())
			return
		}
		logSuccess("Sent user help embed for emoji")
		return
	}

	// which command was invoked?
	switch command[1] {
	case "create":
		logInfo("User invoked create for emoji command")
		// verify correct number of arguments
		if len(command) != 4 {
			attemptSendMsg(s, m, "Usage:\n`emoji create <name> <url>`")
			return
		}

		// verify alphanumeric and underscores
		matched, err := regexp.MatchString(`^[a-zA-Z0-9_]*$`, command[1])
		if err != nil {
			attemptSendMsg(s, m, "Failed to determine whether the name provided was valid.")
			logError("Failed to match regex! " + err.Error())
			return
		}
		if !matched {
			attemptSendMsg(s, m, "Invalid name. Please provide a name using alphanumeric characters or underscores only.")
			return
		}

		// convert image to base64 string
		resp, err := http.Get(command[3])
		if err != nil {
			attemptSendMsg(s, m, "Failed to get a response from the provided URL.")
			logError("No response from URL!" + err.Error())
			return
		}

		defer resp.Body.Close()

		bytes, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			attemptSendMsg(s, m, "Failed to read the response from the provided URL.")
			logError("Couldn't read response from URL!" + err.Error())
			return
		}

		var base64Image string
		mimeType := http.DetectContentType(bytes)

		switch mimeType {
		case "image/jpeg":
			base64Image += "data:image/jpeg;base64,"
		case "image/png":
			base64Image += "data:image/png;base64,"
		case "image/gif":
			base64Image += "data:image/gif;base64,"
		default:
			attemptSendMsg(s, m, "Invalid URL provided. Please provide a jp(e)g, png, or gif image URL.")
			return
		}

		size, err := strconv.Atoi(resp.Header.Get("Content-Length"))
		if err != nil {
			attemptSendMsg(s, m, "Unable to detect file size from provided image URL.")
			return
		}
		downloadSize := int64(size)

		if downloadSize > 262144 {
			attemptSendMsg(s, m, fmt.Sprintf("Please choose an image with smaller file size. Image has a size %.1fKB, which is > 256KB.", float64(downloadSize)/float64(1024.0)))
			logError("Failed to create new emoji due to size constraints!")
			return
		}

		base64Image += base64.StdEncoding.EncodeToString(bytes)

		emoji, err := s.GuildEmojiCreate(m.GuildID, command[2], base64Image, nil)
		if err != nil {
			attemptSendMsg(s, m, fmt.Sprintf("Failed to create new emoji.\n```%s```", err.Error()))
			logError("Failed to create new emoji!" + err.Error())
			return
		}

		attemptSendMsg(s, m, fmt.Sprintf("Created emoji successfully! %s", emoji.MessageFormat()))
		logSuccess("Created new emoji successfully")

	case "delete":
		logInfo("User invoked delete for emoji command")

		// verify correct number of arguments
		if len(command) != 3 {
			attemptSendMsg(s, m, "Usage:\n`emoji delete <emoji>`")
			return
		}

		// validate emoji string formatting
		matched, err := regexp.MatchString(`^(<a?)?:\w+:(\d{18}>)$`, command[2])
		if err != nil {
			attemptSendMsg(s, m, "Failed to determine whether the emoji was valid.")
			logError("Failed to match regex! " + err.Error())
			return
		}
		if !matched {
			attemptSendMsg(s, m, "Invalid argument. Please provide a valid server emoji.")
			logError("Regex did not match!")
			return
		}

		emojiID := strings.TrimSuffix(strings.Split(command[2], ":")[len(strings.Split(command[2], ":"))-1], ">")

		err = s.GuildEmojiDelete(m.GuildID, emojiID)
		if err != nil {
			attemptSendMsg(s, m, fmt.Sprintf("Failed to remove emoji from the server.\n```%s```", err.Error()))
			logError("Failed to remove emoji from the server! " + err.Error())
			return
		}

		attemptSendMsg(s, m, "Removed the emoji from the server.")
		logSuccess("Successfully deleted emoji")

	case "rename":
		logInfo("User invoked rename for emoji command")
		// verify correct number of arguments
		if len(command) != 4 {
			attemptSendMsg(s, m, "Usage:\n`emoji rename <emoji> <new name>`")
			return
		}

		// verify valid emoji formatting provided for 2
		matched, err := regexp.MatchString(`^(<a?)?:\w+:(\d{18}>)$`, command[2])
		if err != nil {
			attemptSendMsg(s, m, "Failed to determine whether the emoji provided was valid.")
			logError("Failed to match regex! " + err.Error())
			return
		}
		if !matched {
			attemptSendMsg(s, m, "Invalid argument. Please provide a valid server emoji.")
			return
		}

		// verify name is alphanumeric for 3
		matched, err = regexp.MatchString(`^[a-zA-Z0-9_]*$`, command[3])
		if err != nil {
			attemptSendMsg(s, m, "Failed to determine whether the name provided was valid.")
			logError("Failed to match regex! " + err.Error())
			return
		}

		if !matched {
			attemptSendMsg(s, m, "Invalid name. Please provide a name using alphanumeric characters or underscores only.")
			return
		}

		emojiID := strings.TrimSuffix(strings.Split(command[2], ":")[len(strings.Split(command[2], ":"))-1], ">")

		// set new name
		_, err = s.GuildEmojiEdit(m.GuildID, emojiID, command[3], nil)
		if err != nil {
			attemptSendMsg(s, m, fmt.Sprintf("Failed to rename the emoji.\n```%s```", command[3]))
			logError("Failed to rename emoji! " + err.Error())
			return
		}

		attemptSendMsg(s, m, fmt.Sprintf("Renamed the emoji to %s.", command[3]))
		logSuccess("Successfully renamed emoji")
		return
	}
}
