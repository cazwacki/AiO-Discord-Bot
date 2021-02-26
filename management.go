package main

import (
	"fmt"
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
	regex := regexp.MustCompile(`^\<\@\!?[0-9]+\>$`)
	if regex.MatchString(command[1]) && len(command) > 2 {
		userID := strings.TrimSuffix(command[1], ">")
		userID = strings.TrimPrefix(userID, "<@")
		userID = strings.TrimPrefix(userID, "!") // this means the user has a nickname
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
	regex := regexp.MustCompile(`^\<\@\!?[0-9]+\>$`)
	if len(command) >= 2 {
		if regex.MatchString(command[1]) {
			userID := strings.TrimSuffix(command[1], ">")
			userID = strings.TrimPrefix(userID, "<@")
			userID = strings.TrimPrefix(userID, "!") // this means the user has a nickname
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
	regex := regexp.MustCompile(`^\<\@\!?[0-9]+\>$`)
	if len(command) >= 2 {
		if regex.MatchString(command[1]) {
			userID := strings.TrimSuffix(command[1], ">")
			userID = strings.TrimPrefix(userID, "<@")
			userID = strings.TrimPrefix(userID, "!") // this means the user has a nickname
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
Attempts to purge the last <number> messages, then removes the purge command.
*/
func attemptPurge(s *discordgo.Session, m *discordgo.MessageCreate, command []string) {
	if len(command) == 2 {
		messageCount, err := strconv.Atoi(command[1])
		if err != nil {
			s.ChannelMessageSend(m.ChannelID, "Usage: `~purge <number> (optional: @user)`")
			return
		}
		if messageCount < 1 {
			s.ChannelMessageSend(m.ChannelID, ":frowning: Sorry, you must purge at least 1 message. Try again.")
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
				s.ChannelMessageSend(m.ChannelID, ":frowning: I couldn't pull messages from the channel. Try again.")
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
			s.ChannelMessagesBulkDelete(m.ChannelID, messageIDs)
			messageCount -= messagesToPurge
		}
		time.Sleep(time.Second)
		s.ChannelMessageDelete(m.ChannelID, m.ID)
	} else if len(command) == 3 {
		// the behavior here is significantly different, so it warrants its own section.
		regex := regexp.MustCompile(`^\<\@\!?[0-9]+\>$`)
		userID := ""
		if regex.MatchString(command[2]) {
			userID = strings.TrimSuffix(command[2], ">")
			userID = strings.TrimPrefix(userID, "<@")
			userID = strings.TrimPrefix(userID, "!") // this means the user has a nickname
		} else {
			s.ChannelMessageSend(m.ChannelID, "Usage: `~purge <number> (optional: @user)`")
			return
		}
		messageCount, err := strconv.Atoi(command[1])
		if err != nil {
			s.ChannelMessageSend(m.ChannelID, "Usage: `~purge <number> (optional: @user)`")
			return
		}
		if messageCount < 1 {
			s.ChannelMessageSend(m.ChannelID, ":frowning: Sorry, you must purge at least 1 message. Try again.")
			return
		}

		// 1. check 100 most recent messages
		// 2. delete messages associated with the given user
		// 3. if i didnt delete the required number of messages, check the next set
		// 4. end conditions: required number of messages are deleted, or there are no more messages in the channel
		currentID := m.ID
		for messageCount > 0 {
			messages, err := s.ChannelMessages(m.ChannelID, 100, currentID, "", "")
			if err != nil {
				s.ChannelMessageSend(m.ChannelID, ":frowning: I couldn't pull messages from the channel. Try again.")
				return
			}

			// get the message IDs for the requested user
			var messageIDs []string
			for _, message := range messages {
				if message.Author.ID == userID && messageCount > 0 {
					messageIDs = append(messageIDs, message.ID)
					messageCount--
				} else {
					currentID = message.ID
				}
			}

			// delete all the marked messages
			s.ChannelMessagesBulkDelete(m.ChannelID, messageIDs)

			if len(messages) < 100 {
				messageCount = 0
			}
		}
		time.Sleep(time.Second)
		s.ChannelMessageDelete(m.ChannelID, m.ID)
	} else {
		s.ChannelMessageSend(m.ChannelID, "Usage: `~purge <number> (optional: @user)`")
	}
}

/**
Attempts to copy over the last <number> messages to the given channel, then outputs its success
*/
func attemptCopy(s *discordgo.Session, m *discordgo.MessageCreate, command []string, preserveMessages bool) {
	var commandInvoked string
	if preserveMessages {
		commandInvoked = "cp"
	} else {
		commandInvoked = "mv"
	}
	if len(command) == 3 {
		messageCount, err := strconv.Atoi(command[1])
		if err != nil {
			s.ChannelMessageSend(m.ChannelID, "Usage: `~"+commandInvoked+" <number <= 100> <#channel>`")
			return
		}

		// verify correctly invoking channel
		if !strings.HasPrefix(command[2], "<#") || !strings.HasSuffix(command[2], ">") {
			s.ChannelMessageSend(m.ChannelID, "Usage: `~"+commandInvoked+" <number <= 100> <#channel>`")
			return
		}
		channel := strings.ReplaceAll(command[2], "<#", "")
		channel = strings.ReplaceAll(channel, ">", "")

		// retrieve messages from current invoked channel
		messages, err := s.ChannelMessages(m.ChannelID, messageCount, m.ID, "", "")
		if err != nil {
			s.ChannelMessageSend(m.ChannelID, "Usage: Ran into an error retrieving messages. :slight_frown:")
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
					fmt.Println(err)
				}
			}

			// populating author information in the embed
			if message.Author != nil {
				member, err := s.GuildMember(m.GuildID, message.Author.ID)
				nickname := ""
				if err == nil {
					nickname = member.Nick
				} else {
					fmt.Println(err)
				}
				embed.Title = ""
				if nickname != "" {
					embed.Title += nickname + " ("
				}
				embed.Title += message.Author.Username + "#" + message.Author.Discriminator
				if nickname != "" {
					embed.Title += ")"
				}
				var thumbnail discordgo.MessageEmbedThumbnail
				thumbnail.URL = message.Author.AvatarURL("")
				embed.Thumbnail = &thumbnail
			}

			// preserve message timestamp
			embed.Timestamp = string(message.Timestamp)
			var contents []*discordgo.MessageEmbedField

			// output message text
			if message.Content != "" {
				embed.Description = "- \"" + message.Content + "\""
			}

			// output attachments
			if len(message.Attachments) > 0 {
				for _, attachment := range message.Attachments {
					contents = append(contents, createField("Attachment: "+attachment.Filename, attachment.ProxyURL, false))
				}
			}

			// output embed contents (up to 10... jesus christ...)
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
			s.ChannelMessageSendEmbed(channel, &embed)
		}
		s.ChannelMessageSend(m.ChannelID, "Copied "+strconv.Itoa(messageCount)+" messages from <#"+m.ChannelID+"> to <#"+channel+">! :smile:")
	} else {
		s.ChannelMessageSend(m.ChannelID, "Usage: `~"+commandInvoked+" <number <= 100> <#channel>`")
	}
}

/**
Helper function for handleProfile. Attempts to retrieve a user's avatar and return it
in an embed.
*/
func attemptProfile(s *discordgo.Session, m *discordgo.MessageCreate, command []string) {
	fmt.Println(command)
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
				s.ChannelMessageSend(m.ChannelID, "Error retrieving the user. :frowning:")
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

			s.ChannelMessageSendEmbed(m.ChannelID, &embed)
		} else {
			s.ChannelMessageSend(m.ChannelID, "Usage: `~profile @user`")
		}
	} else {
		s.ChannelMessageSend(m.ChannelID, "Usage: `~profile @user`")
	}
}

func attemptAbout(s *discordgo.Session, m *discordgo.MessageCreate, command []string) {
	if len(command) == 2 {
		regex := regexp.MustCompile(`^\<\@\!?[0-9]+\>$`)
		if regex.MatchString(command[1]) {
			userID := strings.TrimSuffix(command[1], ">")
			userID = strings.TrimPrefix(userID, "<@")
			userID = strings.TrimPrefix(userID, "!") // this means the user has a nickname

			fmt.Println(command)

			member, err := s.GuildMember(m.GuildID, userID)
			if err != nil {
				s.ChannelMessageSend(m.ChannelID, "Error retrieving the user. :frowning:")
				return
			}

			var embed discordgo.MessageEmbed
			embed.Type = "rich"

			// title the embed
			embed.Title = "About " + member.User.Username + "#" + member.User.Discriminator

			var contents []*discordgo.MessageEmbedField

			joinDate, err := member.JoinedAt.Parse()
			if err != nil {
				s.ChannelMessageSend(m.ChannelID, "Error parsing Discord's dates. :frowning:")
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
				s.ChannelMessageSend(m.ChannelID, "Error retrieving the guild's roles. :frowning:")
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
				fmt.Println("Couldn't send the message... " + err.Error())
			}

		} else {
			s.ChannelMessageSend(m.ChannelID, "Usage: `~about @user`")
			return
		}
	} else {
		s.ChannelMessageSend(m.ChannelID, "Usage: `~about @user`")
	}
}

/**
Outputs the bot's current uptime.
**/
func handleUptime(s *discordgo.Session, m *discordgo.MessageCreate, start []string) {
	fmt.Println(start[0])
	start_time, err := time.Parse("2006-01-02 15:04:05.999999999 -0700 MST", start[0])
	if err != nil {
		s.ChannelMessageSend(m.ChannelID, "Error parsing the date... :frowning:")
	}
	s.ChannelMessageSend(m.ChannelID, ":robot: Uptime: "+time.Since(start_time).Truncate(time.Second/10).String())
}

/**
Forces the bot to exit with code 0. Note that in Heroku the bot will restart automatically.
**/
func handleShutdown(s *discordgo.Session, m *discordgo.MessageCreate, command []string) {
	s.ChannelMessageSend(m.ChannelID, "Shutting Down.")
	s.Close()
	os.Exit(0)
}

/**
Generates an invite code to the channel in which ~invite was invoked if the user has the
permission to create instant invites.
**/
func handleInvite(s *discordgo.Session, m *discordgo.MessageCreate, command []string) {
	if !userHasValidPermissions(s, m, discordgo.PermissionCreateInstantInvite) {
		s.ChannelMessageSend(m.ChannelID, "Sorry, you aren't allowed to create an instant invite.")
		return
	}
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
}

/**
Nicknames the user if they target themselves, or nicknames a target user if the user who invoked
~nick has the permission to change nicknames.
**/
func handleNickname(s *discordgo.Session, m *discordgo.MessageCreate, command []string) {
	if !(userHasValidPermissions(s, m, discordgo.PermissionChangeNickname) && strings.Contains(command[1], m.Author.ID)) && !(userHasValidPermissions(s, m, discordgo.PermissionManageNicknames)) {
		s.ChannelMessageSend(m.ChannelID, "Sorry, you aren't allowed to change nicknames.")
		return
	}
	attemptRename(s, m, command)
}

/**
Kicks a user from the server if the invoking user has the permission to kick users.
**/
func handleKick(s *discordgo.Session, m *discordgo.MessageCreate, command []string) {
	if !userHasValidPermissions(s, m, discordgo.PermissionKickMembers) {
		// validate caller has permission to kick other users
		s.ChannelMessageSend(m.ChannelID, "Sorry, you aren't allowed to kick users.")
		return
	}
	attemptKick(s, m, command)
}

/**
Bans a user from the server if the invoking user has the permission to ban users.
**/
func handleBan(s *discordgo.Session, m *discordgo.MessageCreate, command []string) {
	if !userHasValidPermissions(s, m, discordgo.PermissionBanMembers) {
		// validate caller has permission to kick other users
		s.ChannelMessageSend(m.ChannelID, "Sorry, you aren't allowed to ban users.")
		return
	}
	attemptBan(s, m, command)
}

/**
Removes the <number> most recent messages from the channel where the command was called.
**/
func handlePurge(s *discordgo.Session, m *discordgo.MessageCreate, command []string) {
	if !userHasValidPermissions(s, m, discordgo.PermissionManageMessages) {
		s.ChannelMessageSend(m.ChannelID, "Sorry, you aren't allowed to remove messages.")
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
		s.ChannelMessageSend(m.ChannelID, "Sorry, you aren't allowed to manage messages.")
		return
	}
	attemptCopy(s, m, command, true)
}

/**
Same as above, but purges each message it copies
**/
func handleMove(s *discordgo.Session, m *discordgo.MessageCreate, command []string) {
	if !userHasValidPermissions(s, m, discordgo.PermissionManageMessages) {
		s.ChannelMessageSend(m.ChannelID, "Sorry, you aren't allowed to manage messages.")
		return
	}
	attemptCopy(s, m, command, false)
}

/**
Creates an embed showing a user's profile as a bigger image so it is more visible.
*/
func handleProfile(s *discordgo.Session, m *discordgo.MessageCreate, command []string) {
	attemptProfile(s, m, command)
}

/**
Provides user details in an embed.
*/
func handleAbout(s *discordgo.Session, m *discordgo.MessageCreate, command []string) {
	attemptAbout(s, m, command)
}
