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
		sendError(s, m, "", Discord)
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
		sendError(s, m, commandInvoked, Syntax)
		return
	}

	messageCount, err := strconv.Atoi(command[1])
	if err != nil {
		sendError(s, m, commandInvoked, Syntax)
		return
	}

	// verify correctly invoking channel
	if !strings.HasPrefix(command[2], "<#") || !strings.HasSuffix(command[2], ">") {
		sendError(s, m, commandInvoked, Syntax)
		return
	}

	channel := strings.ReplaceAll(command[2], "<#", "")
	channel = strings.ReplaceAll(channel, ">", "")

	// retrieve messages from current invoked channel
	messages, err := s.ChannelMessages(m.ChannelID, messageCount, m.ID, "", "")
	if err != nil {
		sendError(s, m, commandInvoked, Discord)
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
		embed.Timestamp = message.Timestamp.String()
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

	sendSuccess(s, m, "")
}

/**
Helper function for handleProfile. Attempts to retrieve a user's avatar and return it
in an embed.
*/
func attemptProfile(s *discordgo.Session, m *discordgo.MessageCreate, command []string) {
	logInfo(strings.Join(command, " "))

	if len(command) != 2 {
		sendError(s, m, "profile", Syntax)
		return
	}

	regex := regexp.MustCompile(`^\<\@\!?[0-9]+\>$`)
	if !regex.MatchString(command[1]) {
		sendError(s, m, "profile", Syntax)
		return
	}

	userID := strings.TrimSuffix(command[1], ">")
	userID = strings.TrimPrefix(userID, "<@")
	userID = strings.TrimPrefix(userID, "!") // this means the user has a nickname
	var embed discordgo.MessageEmbed
	embed.Type = "rich"

	// get user
	user, err := s.User(userID)
	if err != nil {
		logError("Could not retrieve user from session! " + err.Error())
		sendError(s, m, "profile", Discord)
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
}

func attemptAbout(s *discordgo.Session, m *discordgo.MessageCreate, command []string) {
	logInfo(strings.Join(command, " "))

	if len(command) != 2 {
		sendError(s, m, "about", Syntax)
		return
	}

	regex := regexp.MustCompile(`^\<\@\!?[0-9]+\>$`)
	if !regex.MatchString(command[1]) {
		sendError(s, m, "about", Syntax)
		return
	}

	userID := stripUserID(command[1])

	logInfo(strings.Join(command, " "))

	member, err := s.GuildMember(m.GuildID, userID)
	if err != nil {
		logError("Could not retrieve user from the session! " + err.Error())
		sendError(s, m, "about", Discord)
		return
	}

	var embed discordgo.MessageEmbed
	embed.Type = "rich"

	// title the embed
	embed.Title = "About " + member.User.Username + "#" + member.User.Discriminator

	var contents []*discordgo.MessageEmbedField

	nickname := "N/A"
	if member.Nick != "" {
		nickname = member.Nick
	}

	contents = append(contents, createField("Server Join Date", member.JoinedAt.Format("01/02/2006"), false))
	contents = append(contents, createField("Nickname", nickname, false))

	// get user's roles in readable form
	guildRoles, err := s.GuildRoles(m.GuildID)
	if err != nil {
		logError("Failed to retrieve guild roles! " + err.Error())
		sendError(s, m, "about", Discord)
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
	logInfo(strings.Join(command, " "))

	if !userHasValidPermissions(s, m, discordgo.PermissionVoiceDeafenMembers) {
		sendError(s, m, "vcdeaf", Permissions)
		return
	}

	if len(command) != 2 {
		sendError(s, m, "vcdeaf", Syntax)
		return
	}

	// 1. get user ID
	regex := regexp.MustCompile(`^\<\@\!?[0-9]+\>$`)
	if !regex.MatchString(command[1]) {
		sendError(s, m, "vcdeaf", Syntax)
		return
	}

	userID := stripUserID(command[1])

	member, err := s.GuildMember(m.GuildID, userID)
	if err != nil {
		logError("Could not retrieve user from the session! " + err.Error())
		sendError(s, m, "vcdeaf", Discord)
		return
	}

	// 2. toggle deafened state
	err = s.GuildMemberDeafen(m.GuildID, userID, !member.Deaf)
	if err != nil {
		logError("Failed to toggle deafened state of the user! " + err.Error())
		sendError(s, m, "vcdeaf", Discord)
		return
	}

	sendSuccess(s, m, "")
}

/**
Toggles "VC muted" state of the specified user.
**/
func vcMute(s *discordgo.Session, m *discordgo.MessageCreate, command []string) {
	if !userHasValidPermissions(s, m, discordgo.PermissionVoiceMuteMembers) {
		sendError(s, m, "vcmute", Permissions)
		return
	}

	if len(command) != 2 {
		sendError(s, m, "vcmute", Syntax)
		return
	}

	// 1. get user ID
	regex := regexp.MustCompile(`^\<\@\!?[0-9]+\>$`)
	if !regex.MatchString(command[1]) {
		sendError(s, m, "vcmute", Syntax)
		return
	}

	userID := stripUserID(command[1])

	logInfo(strings.Join(command, " "))

	member, err := s.GuildMember(m.GuildID, userID)
	if err != nil {
		logError("Could not retrieve user from the session! " + err.Error())
		sendError(s, m, "vcmute", Discord)
		return
	}

	// 2. toggled vc muted state
	err = s.GuildMemberMute(m.GuildID, userID, !member.Mute)
	if err != nil {
		logError("Failed to toggle muted state of the user! " + err.Error())
		sendError(s, m, "vcmute", Discord)
		return
	}

	sendSuccess(s, m, "")
}

/**
Moves the user to the specified voice channel.
**/
func vcMove(s *discordgo.Session, m *discordgo.MessageCreate, command []string) {
	if !userHasValidPermissions(s, m, discordgo.PermissionVoiceDeafenMembers) {
		sendError(s, m, "vcmove", Permissions)
		return
	}

	if len(command) != 3 {
		sendError(s, m, "vcmove", Syntax)
		return
	}

	// 1. get user ID
	regex := regexp.MustCompile(`^\<\@\!?[0-9]+\>$`)
	if !regex.MatchString(command[1]) {
		sendError(s, m, "vcmove", Syntax)
		return
	}

	userID := stripUserID(command[1])

	logInfo(strings.Join(command, " "))

	// 2. get voice channel
	if !strings.HasPrefix(command[2], "<#") || !strings.HasSuffix(command[2], ">") {
		sendError(s, m, "vcmove", Syntax)
		return
	}
	channel := strings.ReplaceAll(command[2], "<#", "")
	channel = strings.ReplaceAll(channel, ">", "")

	// 3. move user to voice channel
	err := s.GuildMemberMove(m.GuildID, userID, &channel)
	if err != nil {
		logError("Failed to move the user to that channel! " + err.Error())
		sendError(s, m, "vcmove", Discord)
		return
	}

	sendSuccess(s, m, "")
}

/**
Kicks the specified user from the voice channel they are in, if any.
**/
func vcKick(s *discordgo.Session, m *discordgo.MessageCreate, command []string) {
	if !userHasValidPermissions(s, m, discordgo.PermissionVoiceMuteMembers) {
		sendError(s, m, "vckick", Permissions)
		return
	}

	if len(command) != 2 {
		sendError(s, m, "vckick", Syntax)
		return
	}

	// 1. get user ID
	regex := regexp.MustCompile(`^\<\@\!?[0-9]+\>$`)
	if !regex.MatchString(command[1]) {
		sendError(s, m, "vckick", Syntax)
		return
	}

	userID := stripUserID(command[1])

	// 2. remove user from voice channel
	err := s.GuildMemberMove(m.GuildID, userID, nil)
	if err != nil {
		logError("Failed to remove the user from that channel! " + err.Error())
		sendError(s, m, "vckick", Discord)
		return
	}

	sendSuccess(s, m, "")
}

/**
Forces the bot to exit with code 0. Note that in Heroku the bot will restart automatically.
**/
func handleShutdown(s *discordgo.Session, m *discordgo.MessageCreate, command []string) {
	logInfo(strings.Join(command, " "))
	if m.Author.ID == "172311520045170688" {
		sendSuccess(s, m, "")
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
		sendError(s, m, "invite", Permissions)
		return
	}
	var invite discordgo.Invite
	invite.Temporary = false
	invite.MaxAge = 21600 // 6 hours
	invite.MaxUses = 0    // infinite uses
	inviteResult, err := s.ChannelInviteCreate(m.ChannelID, invite)
	if err != nil {
		logError("Failed to generate invite! " + err.Error())
		sendError(s, m, "invite", Discord)
		return
	} else {
		attemptSendMsg(s, m, fmt.Sprintf(":mailbox_with_mail: Here's your invitation! https://discord.gg/%s", inviteResult.Code))
	}
}

/**
Nicknames the user if they target themselves, or nicknames a target user if the user who invoked
~nick has the permission to change nicknames.
**/
func handleNickname(s *discordgo.Session, m *discordgo.MessageCreate, command []string) {
	if !(userHasValidPermissions(s, m, discordgo.PermissionChangeNickname) && strings.Contains(command[1], m.Author.ID)) && !(userHasValidPermissions(s, m, discordgo.PermissionManageNicknames)) {
		sendError(s, m, "nick", Permissions)
		return
	}

	logInfo(strings.Join(command, " "))

	if len(command) != 3 {
		sendError(s, m, "nick", Syntax)
		return
	}

	regex := regexp.MustCompile(`^\<\@\!?[0-9]+\>$`)
	if !regex.MatchString(command[1]) {
		sendError(s, m, "nick", Syntax)
		return
	}

	userID := stripUserID(command[1])
	err := s.GuildMemberNickname(m.GuildID, userID, strings.Join(command[2:], " "))
	if err != nil {
		logError("Failed to set nickname! " + err.Error())
		sendError(s, m, "nick", Discord)
		return
	}

	sendSuccess(s, m, "")
}

/**
Kicks a user from the server if the invoking user has the permission to kick users.
**/
func handleKick(s *discordgo.Session, m *discordgo.MessageCreate, command []string) {
	if !userHasValidPermissions(s, m, discordgo.PermissionKickMembers) {
		sendError(s, m, "kick", Permissions)
		return
	}

	if len(command) < 2 {
		sendError(s, m, "kick", Syntax)
		return
	}

	regex := regexp.MustCompile(`^\<\@\!?[0-9]+\>$`)
	if !regex.MatchString(command[1]) {
		sendError(s, m, "kick", Syntax)
		return
	}

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
			logError("Failed to kick user! " + err.Error())
			sendError(s, m, "kick", Discord)
			return
		}

		sendSuccess(s, m, fmt.Sprintf(":wave: Kicked %s for the following reason: '%s'.", command[1], reason))
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
			logError("Failed to kick user! " + err.Error())
			sendError(s, m, "kick", Discord)
			return
		}
		sendSuccess(s, m, fmt.Sprintf(":wave: Kicked %s.", command[1]))
	}
}

/**
Bans a user from the server if the invoking user has the permission to ban users.
**/
func handleBan(s *discordgo.Session, m *discordgo.MessageCreate, command []string) {
	if !userHasValidPermissions(s, m, discordgo.PermissionBanMembers) {
		sendError(s, m, "ban", Permissions)
		return
	}

	if len(command) < 2 {
		sendError(s, m, "ban", Syntax)
		return
	}

	regex := regexp.MustCompile(`^\<\@\!?[0-9]+\>$`)
	if regex.MatchString(command[1]) {
		sendError(s, m, "ban", Syntax)
		return
	}

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
			logError("Failed to ban user! " + err.Error())
			sendError(s, m, "ban", Discord)
			return
		}

		sendSuccess(s, m, fmt.Sprintf(":hammer: Banned %s for the following reason: '%s'.", command[1], reason))
	} else {
		// ban without reason
		err := s.GuildBanCreate(m.GuildID, userID, 0)
		if err != nil {
			logError("Failed to ban user! " + err.Error())
			sendError(s, m, "ban", Discord)
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

		sendSuccess(s, m, fmt.Sprintf(":hammer: Banned %s.", command[1]))
	}
}

/**
Removes the <number> most recent messages from the channel where the command was called.
**/
func handlePurge(s *discordgo.Session, m *discordgo.MessageCreate, command []string) {
	if !userHasValidPermissions(s, m, discordgo.PermissionManageMessages) {
		sendError(s, m, "purge", Permissions)
		return
	}

	if len(command) != 2 {
		sendError(s, m, "purge", Syntax)
		return
	}

	messageCount, err := strconv.Atoi(command[1])
	if err != nil {
		sendError(s, m, "purge", Syntax)
		return
	}

	if messageCount < 1 {
		logWarning("User attempted to purge < 1 message.")
		attemptSendMsg(s, m, ":frowning: Sorry, you must purge at least 1 message. Try again.")
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
			logError("Failed to pull messages from channel! " + err.Error())
			attemptSendMsg(s, m, ":frowning: I couldn't pull messages from the channel. Try again.")
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
}

/**
Copies the <number> most recent messages from the channel where the command was called and
pastes it in the requested channel.
**/
func handleCopy(s *discordgo.Session, m *discordgo.MessageCreate, command []string) {
	if !userHasValidPermissions(s, m, discordgo.PermissionManageMessages) {
		sendError(s, m, "cp", Permissions)
		return
	}
	attemptCopy(s, m, command, true)
}

/**
Same as above, but purges each message it copies
**/
func handleMove(s *discordgo.Session, m *discordgo.MessageCreate, command []string) {
	if !userHasValidPermissions(s, m, discordgo.PermissionManageMessages) {
		sendError(s, m, "mv", Permissions)
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
		sendError(s, m, "emoji", Permissions)
		return
	}

	if len(command) < 2 {
		sendError(s, m, "emoji", Syntax)
		return
	}

	// which command was invoked?
	switch command[1] {
	case "help":

		if len(command) != 2 {
			sendError(s, m, "emoji", Syntax)
		}

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
	case "create":
		// verify correct number of arguments
		if len(command) != 4 {
			sendError(s, m, "emoji", Syntax)
			return
		}

		// verify alphanumeric and underscores
		matched, err := regexp.MatchString(`^[a-zA-Z0-9_]*$`, command[1])
		if err != nil {
			logError("Failed to match regex! " + err.Error())
			sendError(s, m, "emoji", Internal)
			return
		}
		if !matched {
			attemptSendMsg(s, m, "Invalid name. Please provide a name using alphanumeric characters or underscores only.")
			return
		}

		// convert image to base64 string
		resp, err := http.Get(command[3])
		if err != nil {
			logError("No response from URL!" + err.Error())
			sendError(s, m, "emoji", ReadParse)
			return
		}

		defer resp.Body.Close()

		bytes, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			logError("Couldn't read response from URL!" + err.Error())
			sendError(s, m, "emoji", ReadParse)
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
			logError("Failed to create new emoji due to size constraints!")
			attemptSendMsg(s, m, fmt.Sprintf("Please choose an image with smaller file size. Image has a size %.1fKB, which is > 256KB.", float64(downloadSize)/float64(1024.0)))
			return
		}

		base64Image += base64.StdEncoding.EncodeToString(bytes)

		_, err = s.GuildEmojiCreate(m.GuildID, command[2], base64Image, nil)
		if err != nil {
			logError("Failed to create new emoji!" + err.Error())
			sendError(s, m, "emoji", Discord)
			return
		}

		sendSuccess(s, m, "")

	case "delete":
		// verify correct number of arguments
		if len(command) != 3 {
			sendError(s, m, "emoji", Syntax)
			return
		}

		// validate emoji string formatting
		matched, err := regexp.MatchString(`^(<a?)?:\w+:(\d{18}>)$`, command[2])
		if err != nil {
			logError("Failed to match regex! " + err.Error())
			sendError(s, m, "emoji", Internal)
			return
		}
		if !matched {
			logError("Regex did not match!")
			sendError(s, m, "emoji", Syntax)
			return
		}

		emojiID := strings.TrimSuffix(strings.Split(command[2], ":")[len(strings.Split(command[2], ":"))-1], ">")

		err = s.GuildEmojiDelete(m.GuildID, emojiID)
		if err != nil {
			logError("Failed to remove emoji from the server! " + err.Error())
			sendError(s, m, "emoji", Discord)
			return
		}

		sendSuccess(s, m, "")

	case "rename":
		// verify correct number of arguments
		if len(command) != 4 {
			sendError(s, m, "emoji", Syntax)
			return
		}

		// verify valid emoji formatting provided for 2
		matched, err := regexp.MatchString(`^(<a?)?:\w+:(\d{18}>)$`, command[2])
		if err != nil {
			logError("Failed to match regex! " + err.Error())
			sendError(s, m, "emoji", Internal)
			return
		}
		if !matched {
			sendError(s, m, "emoji", Syntax)
			return
		}

		// verify name is alphanumeric for 3
		matched, err = regexp.MatchString(`^[a-zA-Z0-9_]*$`, command[3])
		if err != nil {
			logError("Failed to match regex! " + err.Error())
			sendError(s, m, "emoji", Internal)
			return
		}
		if !matched {
			sendError(s, m, "emoji", Syntax)
			return
		}

		emojiID := strings.TrimSuffix(strings.Split(command[2], ":")[len(strings.Split(command[2], ":"))-1], ">")

		// set new name
		_, err = s.GuildEmojiEdit(m.GuildID, emojiID, command[3], nil)
		if err != nil {
			logError("Failed to rename emoji! " + err.Error())
			sendError(s, m, "emoji", Discord)
			return
		}

		sendSuccess(s, m, "")
	}
}
