package main

import (
	"database/sql"
	"fmt"
	"math"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
	_ "github.com/go-sql-driver/mysql"
)

var connection_pool *sql.DB
var dbUsername string
var dbPassword string
var dbName string
var dbHost string
var activityTable string
var leaderboardTable string
var joinLeaveTable string
var autokickTable string
var modLogTable string
var autoshrineTable string

type AutoKickData struct {
	GuildID       string `json:"guild_id"`
	DaysUntilKick int    `json:"days_until_kick"`
}

type ModLogData struct {
	GuildID   string `json:"guild_id"`
	ChannelID string `json:"channel_id"`
}

type MemberActivity struct {
	ID          int    `json:"entry"`
	GuildID     string `json:"guild_id"`
	MemberID    string `json:"member_id"`
	MemberName  string `json:"member_name"`
	LastActive  string `json:"last_active"`
	Description string `json:"description"`
	Whitelisted int    `json:"whitelist"`
}

type LeaderboardEntry struct {
	ID          int    `json:"entry"`
	GuildID     string `json:"guild_id"`
	MemberID    string `json:"member_id"`
	MemberName  string `json:"member_name"`
	Points      int    `json:"points"`
	LastAwarded string `json:"last_awarded"`
}

type GreeterMessage struct {
	ID          int    `json:"entry"`
	GuildID     string `json:"guild_id"`
	ChannelID   string `json:"channel_id"`
	MessageType string `json:"message_type"`
	ImageLink   string `json:"image_link"`
	Message     string `json:"message"`
}

type InactiveSet struct {
	DaysInactive int
	Message      *discordgo.Message
	Inactives    []MemberActivity
	Index        int
}

/****
EVENT HANDLERS
****/
// logs on - updating the server (like icon, name) - create / update / delete a channel - kick member - ban / unban member - emoji create / update / delete
func logModActivity(s *discordgo.Session, guildID string, entry *discordgo.AuditLogEntry) {
	query, err := connection_pool.Query(fmt.Sprintf("SELECT * FROM %s WHERE (guild_id = ?);", modLogTable), guildID)
	if err != nil {
		logError("SELECT query error, so stopping execution: " + err.Error())
		return
	}
	defer query.Close()

	for query.Next() {
		// write and send embed
		var modLogData ModLogData
		err = query.Scan(&modLogData.GuildID, &modLogData.ChannelID)
		if err != nil {
			logError("Unable to parse database information! Aborting. " + err.Error())
			return
		}

		var embed discordgo.MessageEmbed
		embed.Type = "rich"

		switch *entry.ActionType {
		case discordgo.AuditLogActionMemberKick, discordgo.AuditLogActionMemberBanAdd, discordgo.AuditLogActionMemberBanRemove:
			user, err := s.User(entry.TargetID)
			if err != nil {
				logError("Unable to get user from session state!")
				return
			}
			actor, err := s.GuildMember(guildID, entry.UserID)
			if err != nil {
				logError("Unable to get actor from session state!")
				return
			}
			action := ""
			switch *entry.ActionType {
			case discordgo.AuditLogActionMemberKick:
				action = "👢Kicked"
			case discordgo.AuditLogActionMemberBanAdd:
				action = "🚫Banned"
			case discordgo.AuditLogActionMemberBanRemove:
				action = "🤝Ban Revoked for"
			}
			embed.Title = fmt.Sprintf("%s %s#%s", action, user.Username, user.Discriminator)
			actorString := fmt.Sprintf("%s#%s", actor.User.Username, actor.User.Discriminator)
			if actor.Nick != "" {
				actorString = fmt.Sprintf("%s (%s)", actor.Nick, actorString)
			}
			embed.Description = fmt.Sprintf("**Actor**: %s\n", actorString)
			if *entry.ActionType != discordgo.AuditLogActionMemberBanRemove {
				embed.Description += fmt.Sprintf("**Reason**: '%s'", entry.Reason)
			}

			var thumbnail discordgo.MessageEmbedThumbnail
			thumbnail.URL = user.AvatarURL("512")
			embed.Thumbnail = &thumbnail
		}

		_, err := s.ChannelMessageSendEmbed(modLogData.ChannelID, &embed)
		if err != nil {
			logError("Failed to send message embed. " + err.Error())
		}
	}
}

// logs when a user sends a message, reacts to a message, or joins the server.
func logActivity(guildID string, user *discordgo.User, time string, description string, newUser bool) {
	if user.Bot {
		return
	}

	description = strings.ReplaceAll(description, "'", "''")

	if len(description) > 80 {
		description = description[0:80]
	}

	if newUser {
		// INSERT INTO table (guild_id, member_id, last_active, description) VALUES (guildID, userID, time, description)
		attemptQuery(fmt.Sprintf("INSERT INTO %s (guild_id, member_id, member_name, last_active, description) VALUES (?, ?, ?, ?, ?);", activityTable),
			"New user added to activity log",
			"Unable to insert new user!",
			guildID, user.ID, strings.ReplaceAll(user.Username, "'", "\\'")+"#"+user.Discriminator, time, description)
	} else {
		// UPDATE table SET (last_active = time, description = description) WHERE (guild_id = guildID AND member_id = userID)
		attemptQuery(fmt.Sprintf("UPDATE %s SET last_active = ?, description = ?, member_name = ? WHERE (guild_id = ? AND member_id = ?);", activityTable),
			"User's activity updated",
			"Unable to update user's activity!",
			time, description, strings.ReplaceAll(user.Username, "'", "\\'")+"#"+user.Discriminator, guildID, user.ID)
	}

}

// removes the user's row when they leave the server.
func removeUser(guildID string, userID string) {
	attemptQuery(fmt.Sprintf("DELETE FROM %s WHERE (guild_id = ? AND member_id = ?);", activityTable),
		"User removed from activity log",
		"Couldn't remove user from activity log",
		guildID, userID)
}

// sends the guild's join/leave message when a user enters/leaves the server.
func joinLeaveMessage(s *discordgo.Session, guildID string, user *discordgo.User, messageType string) {
	query, err := connection_pool.Query(fmt.Sprintf("SELECT * FROM %s WHERE (guild_id = ? AND message_type = ?);", joinLeaveTable), guildID, messageType)
	if err != nil {
		logError("SELECT query error, but not stopping execution: " + err.Error())
	}
	defer query.Close()

	guild, err := s.State.Guild(guildID)
	if err != nil {
		logError("Unable to retrieve the guild from session state! " + err.Error())
		return
	}

	sUser := user.Username
	sDisc := user.Discriminator
	sPing := fmt.Sprintf("<@%s>", user.ID)
	sMemc := guild.MemberCount

	for query.Next() {
		// write and send embed
		var greeterMessage GreeterMessage
		err = query.Scan(&greeterMessage.ID, &greeterMessage.GuildID, &greeterMessage.ChannelID, &greeterMessage.MessageType, &greeterMessage.ImageLink, &greeterMessage.Message)
		if err != nil {
			logError("Unable to parse database information! Aborting. " + err.Error())
			return
		}

		// do all code substitutions
		greeterMessage.Message = strings.ReplaceAll(greeterMessage.Message, "<<user>>", sUser)
		greeterMessage.Message = strings.ReplaceAll(greeterMessage.Message, "<<disc>>", sDisc)
		greeterMessage.Message = strings.ReplaceAll(greeterMessage.Message, "<<ping>>", sPing)
		greeterMessage.Message = strings.ReplaceAll(greeterMessage.Message, "<<memc>>", strconv.Itoa(sMemc))

		var embed discordgo.MessageEmbed
		embed.Type = "rich"
		embed.Description = greeterMessage.Message

		var image discordgo.MessageEmbedImage
		image.URL = greeterMessage.ImageLink
		embed.Image = &image

		_, err := s.ChannelMessageSendEmbed(greeterMessage.ChannelID, &embed)
		if err != nil {
			logError("Failed to send message embed. " + err.Error())
		}
	}
}

// loads the provided guild's members into the database.
func logNewGuild(s *discordgo.Session, guildID string) int {

	// loop through members and populate our list of users in the guild
	var memberList []*discordgo.Member
	after := ""
	scanning := true
	for scanning {
		nextMembers, err := s.GuildMembers(guildID, after, 500)
		if err != nil {
			logError("Unable to scan the full guild! " + err.Error())
			return 0
		}
		if len(nextMembers) < 1000 {
			scanning = false
		}
		memberList = append(memberList, nextMembers...)
		after = memberList[len(memberList)-1].User.ID
	}

	results, err := connection_pool.Query(fmt.Sprintf("SELECT * FROM %s WHERE (guild_id = ?)", activityTable), guildID)
	if err != nil {
		logError("Unable to read database for existing users in the guild! " + err.Error())
		return 0
	}
	defer results.Close()

	// loop through members in the database and store them in an array
	var memberActivities []MemberActivity
	for results.Next() {
		var memberActivity MemberActivity
		err = results.Scan(&memberActivity.ID, &memberActivity.GuildID, &memberActivity.MemberID, &memberActivity.MemberName, &memberActivity.LastActive, &memberActivity.Description, &memberActivity.Whitelisted)
		if err != nil {
			logError("Unable to parse database information! Aborting. " + err.Error())
			return 0
		}
		memberActivities = append(memberActivities, memberActivity)
	}

	membersAddedToDatabase := 0
	for _, member := range memberList {
		if !member.User.Bot {
			memberExistsInDatabase := false
			for _, memberActivity := range memberActivities {
				if memberActivity.MemberID == member.User.ID {
					memberExistsInDatabase = true
				}
			}
			if !memberExistsInDatabase {
				logInfo("Added " + member.User.ID + "to the activity database for guild " + guildID)
				go logActivity(guildID, member.User, time.Now().String(), "Detected in a scan", true)
				membersAddedToDatabase++
			}
		}
	}
	return membersAddedToDatabase

}

// removes the provided guild's members from the database.
func removeGuild(guildID string) {
	attemptQuery(fmt.Sprintf("DELETE FROM %s WHERE (guild_id = ?);", activityTable),
		"Guild removed from activity log",
		"Couldn't remove guild from activity log! Is the connection still available?",
		guildID)
}

// awards a user points for the guild's leaderboard based on the word count formula.
func awardPoints(guildID string, user *discordgo.User, currentTime string, message string) {
	if user.Bot {
		return
	}

	// 1. calculate points from sentence
	wordCount := len(strings.Split(message, " "))

	pointsToAward := int(math.Floor(math.Pow(float64(wordCount), float64(1)/3)*10 - 10))

	query, err := connection_pool.Query(fmt.Sprintf("SELECT * FROM %s WHERE (guild_id = ? AND member_id = ?);", leaderboardTable), guildID, user.ID)
	if err != nil {
		logError("SELECT query error: " + err.Error())
		return
	}
	defer query.Close()

	foundUser := false
	for query.Next() {
		foundUser = true
		var leaderboardEntry LeaderboardEntry
		err = query.Scan(&leaderboardEntry.ID, &leaderboardEntry.GuildID, &leaderboardEntry.MemberID, &leaderboardEntry.MemberID, &leaderboardEntry.Points, &leaderboardEntry.LastAwarded)
		if err != nil {
			logError("Unable to parse database information! Aborting. " + err.Error())
			return
		} else {
			dateFormat := "2006-01-02 15:04:05.999999999 -0700 MST"
			lastAwarded, err := time.Parse(dateFormat, strings.Split(leaderboardEntry.LastAwarded, " m=")[0])
			if err != nil {
				logError("Unable to parse database timestamps! Aborting. " + err.Error())
				return
			}
			lastAwarded = lastAwarded.Add(time.Second * 3)
			if lastAwarded.Before(time.Now()) {
				// add points
				newScore := pointsToAward + leaderboardEntry.Points
				attemptQuery(fmt.Sprintf("UPDATE %s SET last_awarded = ?, points = ?, member_name = ? WHERE (guild_id = ? AND member_id = ?);", leaderboardTable),
					"User points updated",
					"Couldn't update user's points! Is the connection still available?",
					currentTime, newScore, strings.ReplaceAll(user.Username, "'", "\\'")+"#"+user.Discriminator, guildID, user.ID)
			}
		}
	}

	if !foundUser {
		attemptQuery(fmt.Sprintf("INSERT INTO %s (guild_id, member_id, member_name, points, last_awarded) VALUES (?, ?, ?, ?, ?);", leaderboardTable),
			"Added new user to leaderboard",
			"Couldn't add new user to leaderboard! Is the connection still available?",
			guildID, user.ID, strings.ReplaceAll(user.Username, "'", "\\'")+"#"+user.Discriminator, pointsToAward, currentTime)
	}
}

// helper function for queries we don't need the results for.
func attemptQuery(sql string, successMessage string, errMessage string, args ...interface{}) bool {
	query, err := connection_pool.Query(sql, args...)
	if err != nil {
		logError(errMessage + " " + err.Error())
		return false
	}
	defer query.Close()
	logSuccess(successMessage)
	return true
}

/****
COMMANDS
****/
func setModLogChannel(s *discordgo.Session, m *discordgo.MessageCreate, command []string) {
	if !userHasValidPermissions(s, m, discordgo.PermissionManageServer) {
		sendError(s, m, "modlog", Permissions)
		return
	}
	if len(command) != 3 && len(command) != 2 {
		sendError(s, m, "modlog", Syntax)
		return
	}

	switch command[1] {
	case "set":
		// create or update entry in database
		channel := strings.ReplaceAll(command[2], "<#", "")
		channel = strings.ReplaceAll(channel, ">", "")
		matched, _ := regexp.MatchString(`^[0-9]{18}$`, channel)
		if !matched {
			logInfo("User did not specify channel correctly")
			sendError(s, m, "modlog", Syntax)
			return
		}

		// remove old channel if it exists
		if !attemptQuery(fmt.Sprintf("DELETE FROM %s WHERE (guild_id = ?);", modLogTable),
			"Removed old modlog channel",
			"Couldn't remove old modlog channel! Is the connection still available?",
			m.GuildID) {
			sendError(s, m, "modlog", Database)
			return
		}

		// add new channel
		if attemptQuery(fmt.Sprintf("INSERT INTO %s (guild_id, channel_id) VALUES (?, ?);", modLogTable),
			"Added new modlog channel",
			"Couldn't add new modlog channel! Is the connection still available?",
			m.GuildID, channel) {
			sendSuccess(s, m, "")
		} else {
			sendError(s, m, "modlog", Database)
		}

	case "reset":
		// remove old channel if it exists
		if attemptQuery(fmt.Sprintf("DELETE FROM %s WHERE (guild_id = ?);", modLogTable),
			"Removed old modlog channel",
			"Couldn't remove old modlog channel! Is the connection still available?",
			m.GuildID) {
			sendSuccess(s, m, "")
		} else {
			sendError(s, m, "modlog", Database)
		}
	}
}

func greeter(s *discordgo.Session, m *discordgo.MessageCreate, command []string) {
	logInfo(strings.Join(command, " "))
	if !userHasValidPermissions(s, m, discordgo.PermissionManageServer) {
		sendError(s, m, "greeter", Permissions)
		return
	}

	if len(command) == 1 {
		sendError(s, m, "greeter", Syntax)
		return
	}

	switch command[1] {
	case "help":
		var embed discordgo.MessageEmbed
		embed.Type = "rich"
		embed.Title = "Greeter Commands"
		embed.Description = "The greeter has a few codes you can use to substitute server / user data in your message!\n```Codes:\n\t<<user>> -> username\n\t<<disc>> -> discriminator\n\t<<ping>> -> @user\n\t<<memc>> -> member count```\nExample:\n`Welcome, <<ping>>! <<user>>#<<disc>> is member <<memc>> on the server!` becomes `Welcome, @sage! sage#5429 is member 53 on the server!`"

		var contents []*discordgo.MessageEmbedField
		contents = append(contents, createField("~greeter help", "Explains how to use the codes and different commands.", false))
		contents = append(contents, createField("~greeter status", "Displays the current welcome and goodbye messages' information, if present.", false))
		contents = append(contents, createField("~greeter set (join/leave) #channel message(max: 1000 characters) (optional: -img (image URL (max 1000 characters)))", "Adds (or updates) an entry for the guild to send the new message when a user joins/leaves.", false))
		contents = append(contents, createField("~greeter reset (join/leave)", "Removes the join/leave message completely. It will no longer send the corresponding message until you set a message again using `~greeter set`.", false))
		embed.Fields = contents

		_, err := s.ChannelMessageSendEmbed(m.ChannelID, &embed)
		if err != nil {
			logError("Failed to send instructions message embed! " + err.Error())
			sendError(s, m, "greeter", Discord)
			return
		}
	case "status":
		query, err := connection_pool.Query(fmt.Sprintf("SELECT * FROM %s WHERE (guild_id = ?);", joinLeaveTable), m.GuildID)
		if err != nil {
			logWarning("SELECT query error, but not stopping execution: " + err.Error())
		}
		defer query.Close()

		postedMessages := false
		for query.Next() {
			postedMessages = true
			var greeterMessage GreeterMessage
			err = query.Scan(&greeterMessage.ID, &greeterMessage.GuildID, &greeterMessage.ChannelID, &greeterMessage.MessageType, &greeterMessage.ImageLink, &greeterMessage.Message)
			if err != nil {
				logError("Unable to parse database information! Aborting. " + err.Error())
				sendError(s, m, "greeter", Database)
				return
			}

			var embed discordgo.MessageEmbed
			embed.Type = "rich"
			embed.Title = fmt.Sprintf("%s Message", strings.Title(greeterMessage.MessageType))

			var contents []*discordgo.MessageEmbedField
			contents = append(contents, createField("Message", greeterMessage.Message, false))
			contents = append(contents, createField("Channel", fmt.Sprintf("<#%s>", greeterMessage.ChannelID), false))
			imageLink := "N/A"
			if greeterMessage.ImageLink != "" {
				imageLink = greeterMessage.ImageLink
			}
			contents = append(contents, createField("Image", imageLink, false))
			embed.Fields = contents

			_, err := s.ChannelMessageSendEmbed(m.ChannelID, &embed)
			if err != nil {
				logError("Failed to send a greeter status message! " + err.Error())
				return
			}
		}

		if !postedMessages {
			attemptSendMsg(s, m, "This server currently has no greeter messages!")
		}

	case "set":
		if len(command) < 5 {
			sendError(s, m, "greeter", Syntax)
			return
		}
		if command[2] != "join" && command[2] != "leave" {
			logInfo("User did not use 'join' or 'leave' when calling greeter set")
			sendError(s, m, "greeter", Syntax)
			return
		}

		channel := strings.ReplaceAll(command[3], "<#", "")
		channel = strings.ReplaceAll(channel, ">", "")
		matched, _ := regexp.MatchString(`^[0-9]{18}$`, channel)
		if !matched {
			logInfo("User did not specify channel correctly")
			sendError(s, m, "greeter", Syntax)
			return
		}

		message := ""
		imageURL := ""
		if command[len(command)-2] == "-img" {
			message = strings.Join(command[4:len(command)-2], " ")
			imageURL = command[len(command)-1]
		} else {
			message = strings.Join(command[4:], " ")
		}
		message = strings.ReplaceAll(message, "'", "\\'")
		imageURL = strings.ReplaceAll(imageURL, "'", "\\'")

		// remove old message if it exists
		if !attemptQuery(fmt.Sprintf("DELETE FROM %s WHERE (guild_id = ? AND message_type = ?);", joinLeaveTable),
			fmt.Sprintf("Deleted old %s!", command[2]),
			fmt.Sprintf("Unable to delete old %s!", command[2]),
			m.GuildID, command[2]) {
			sendError(s, m, "greeter", Database)
			return
		}

		// add new message
		if attemptQuery(fmt.Sprintf("INSERT INTO %s (guild_id, channel_id, message_type, image_link, message) VALUES (?, ?, ?, ?, ?);", joinLeaveTable),
			fmt.Sprintf("Added new %s!", command[2]),
			fmt.Sprintf("Unable to add new %s!", command[2]),
			m.GuildID, channel, command[2], imageURL, message) {
			sendSuccess(s, m, "")
		} else {
			sendError(s, m, "greeter", Database)
		}
	case "reset":
		if len(command) != 3 {
			sendError(s, m, "greeter", Syntax)
			return
		}

		if command[2] != "join" && command[2] != "leave" {
			logInfo("User did not specify whether the join or leave message was being reset")
			sendError(s, m, "greeter", Syntax)
			return
		}

		if attemptQuery(fmt.Sprintf("DELETE FROM %s WHERE (guild_id = ? AND message_type = ?);", joinLeaveTable),
			"Deleted the message!",
			"Failed to delete the message",
			m.GuildID, command[2]) {
			sendSuccess(s, m, "")
		} else {
			sendError(s, m, "greeter", Database)
		}
	default:
		sendError(s, m, "greeter", Syntax)
	}
}

func leaderboard(s *discordgo.Session, m *discordgo.MessageCreate, command []string) {
	logInfo(strings.Join(command, " "))
	if len(command) > 1 {
		sendError(s, m, "leaderboard", Syntax)
		return
	}

	if len(command) == 1 {
		// generate leaderboard of top 10 users with corresponding points, with user's score at the bottom

		// 1. Get all members of the guild the command was invoked in and sort by points
		results, err := connection_pool.Query(fmt.Sprintf("SELECT * FROM %s WHERE (guild_id = ?);", leaderboardTable), m.GuildID)
		if err != nil {
			sendError(s, m, "leaderboard", Database)
			return
		}
		defer results.Close()

		// create array of users
		var leaderboardEntries []LeaderboardEntry
		for results.Next() {
			var entry LeaderboardEntry
			err = results.Scan(&entry.ID, &entry.GuildID, &entry.MemberID, &entry.MemberName, &entry.Points, &entry.LastAwarded)
			if err != nil {
				logError("Unable to parse database information! Aborting. " + err.Error())
				sendError(s, m, "leaderboard", Database)
				return
			}
			leaderboardEntries = append(leaderboardEntries, entry)
		}

		// sort by points
		sort.Slice(leaderboardEntries, func(i, j int) bool {
			return leaderboardEntries[i].Points > leaderboardEntries[j].Points
		})

		message := "```perl\n"
		// 2. Create a for loop codesnippet message showing the names and ranks of top 10s
		for i := 0; i < len(leaderboardEntries) && i < 10; i++ {
			message += fmt.Sprintf("%d.\t%s\n\t\tPoints: %d\n", (i + 1), leaderboardEntries[i].MemberName, leaderboardEntries[i].Points)
		}

		var authorEntry LeaderboardEntry
		position := 0
		for i := range leaderboardEntries {
			if leaderboardEntries[i].MemberID == m.Author.ID {
				authorEntry = leaderboardEntries[i]
				position = i + 1
			}
		}
		message += "----------------------------------------\nYour Position:\n"
		message += fmt.Sprintf("%d. %s\n\tPoints: %d\n```", position, authorEntry.MemberName, authorEntry.Points)

		// 3. send leaderboard
		attemptSendMsg(s, m, message)
	}
}

func activity(s *discordgo.Session, m *discordgo.MessageCreate, command []string) {
	if len(command) == 1 {
		sendError(s, m, "activity", Syntax)
		return
	}
	logInfo(strings.Join(command, " "))
	switch command[1] {
	case "help":
		var embed discordgo.MessageEmbed
		embed.Type = "rich"
		embed.Title = "Activity Commands"
		embed.Description = "The activity commands allow you to track the activity of your members at a high level, see when they last performed an action in the server, and allow you to automatically kick users that haven't taken actions in a number of days!"

		var contents []*discordgo.MessageEmbedField
		contents = append(contents, createField("~activity help", "Explains how to use the different commands.", false))
		contents = append(contents, createField("~activity rescan", "Displays the current welcome and goodbye messages' information, if present.", false))
		contents = append(contents, createField("~activity user (@user)", "Shows the last action taken by the pinged user in the server.", false))
		contents = append(contents, createField("~activity list (number)", "Lists the users who haven't been active in the last (number) days. If 0 is passed in, it shows all user's activities on the server.", false))
		contents = append(contents, createField("~activity autokick (number)", "Automatically kicks users who haven't been active in the last (number) days. If 0, disables autokicking.", false))
		contents = append(contents, createField("~activity whitelist (@user) (true/false)", "Enables / disables the pinged user's immunity to the autokick functionality.", false))
		embed.Fields = contents

		_, err := s.ChannelMessageSendEmbed(m.ChannelID, &embed)
		if err != nil {
			logError("Failed to send instructions message embed! " + err.Error())
			sendError(s, m, "greeter", Discord)
			return
		}
	case "rescan":
		if len(command) != 2 {
			sendError(s, m, "activity", Syntax)
			return
		}
		sendSuccess(s, m, "")
	case "user":
		if len(command) != 3 {
			sendError(s, m, "activity", Syntax)
			return
		}
		regex := regexp.MustCompile(`^\<\@\!?[0-9]+\>$`)
		if regex.MatchString(command[2]) {
			userID := stripUserID(command[2])

			// parse userID, get it from the db, present info
			query, err := connection_pool.Query(fmt.Sprintf("SELECT * FROM %s WHERE (guild_id = ? AND member_id = ?);", activityTable), m.GuildID, userID)
			if err == sql.ErrNoRows {
				logWarning("User not found in the database. This usually should not happen.")
				sendError(s, m, "activity", Database)
				return
			}
			defer query.Close()

			for query.Next() {
				var memberActivity MemberActivity
				err = query.Scan(&memberActivity.ID, &memberActivity.GuildID, &memberActivity.MemberID, &memberActivity.MemberName, &memberActivity.LastActive, &memberActivity.Description, &memberActivity.Whitelisted)
				if err != nil {
					logError("Unable to parse database information! Aborting. " + err.Error())
					sendError(s, m, "activity", Database)
					return
				}
				dateFormat := "2006-01-02 15:04:05.999999999 -0700 MST"
				lastActive, err := time.Parse(dateFormat, strings.Split(memberActivity.LastActive, " m=")[0])
				if err != nil {
					logError("Unable to parse database timestamps! Aborting. " + err.Error())
					sendError(s, m, "activity", Database)
					return
				}

				var embed discordgo.MessageEmbed
				embed.Type = "rich"
				embed.Title = memberActivity.MemberName
				embed.Description = "- " + lastActive.Format("01/02/2006 15:04:05") + "\n- " + memberActivity.Description

				if memberActivity.Whitelisted == 1 {
					embed.Description += "\n- Protected from auto-kick"
				}

				member, err := s.GuildMember(m.GuildID, userID)
				if err != nil {
					logError("Couldn't pull member information from the session. " + err.Error())
					sendError(s, m, "activity", Discord)
					return
				}
				var thumbnail discordgo.MessageEmbedThumbnail
				thumbnail.URL = member.User.AvatarURL("")
				embed.Thumbnail = &thumbnail

				_, err = s.ChannelMessageSendEmbed(m.ChannelID, &embed)
				if err != nil {
					logError("Failed to send user activity message! " + err.Error())
					return
				}
			}
		}
	case "list":
		if len(command) != 3 {
			sendError(s, m, "activity", Syntax)
			return
		}
		inactiveUsers := getInactiveUsers(s, m, command)
		daysOfInactivity, err := strconv.Atoi(command[2])
		if err != nil {
			logError("Failed to convert string passed in to a number! " + err.Error())
			sendError(s, m, "activity", Syntax)
			return
		}

		if len(inactiveUsers) == 0 {
			attemptSendMsg(s, m, "No user has been inactive for "+strconv.Itoa(daysOfInactivity)+"+ days.")
			return
		}

		var newSet InactiveSet
		newSet.DaysInactive = daysOfInactivity
		newSet.Index = 0
		newSet.Inactives = inactiveUsers

		// craft response and send
		var embed discordgo.MessageEmbed
		embed.Type = "rich"
		if daysOfInactivity > 0 {
			embed.Title = "Users Inactive for " + strconv.Itoa(daysOfInactivity) + "+ Days"
		} else {
			embed.Title = "User Activity"
		}

		var contents []*discordgo.MessageEmbedField
		for i := 0; i < 8 && i < len(inactiveUsers); i++ {
			// calculate difference between time.Now() and the provided timestamp
			dateFormat := "2006-01-02 15:04:05.999999999 -0700 MST"
			lastActive, err := time.Parse(dateFormat, strings.Split(inactiveUsers[i].LastActive, " m=")[0])
			if err != nil {
				logError("Unable to parse database timestamps! Aborting. " + err.Error())
				sendError(s, m, "activity", Database)
				return
			}
			fieldValue := "- " + lastActive.Format("01/02/2006 15:04:05") + "\n- " + inactiveUsers[i].Description
			// add whitelist state
			if inactiveUsers[i].Whitelisted == 1 {
				fieldValue += "\n- Protected from auto-kick"
			}
			contents = append(contents, createField(inactiveUsers[i].MemberName, fieldValue, false))
		}
		embed.Fields = contents

		var footer discordgo.MessageEmbedFooter
		pageCount := len(newSet.Inactives) / 8
		if len(newSet.Inactives)%8 != 0 {
			pageCount++
		}
		logInfo("Page Count: " + strconv.Itoa(pageCount))

		footer.Text = fmt.Sprintf("Page 1 of %d", pageCount)
		embed.Footer = &footer
		message, err := s.ChannelMessageSendEmbed(m.ChannelID, &embed)
		if err != nil {
			logError("Failed to send activity list message! " + err.Error())
			return
		}
		newSet.Message = message
		go appendToGlobalInactiveSet(s, newSet)

		err = s.MessageReactionAdd(m.ChannelID, message.ID, "◀️")
		if err != nil {
			logError("Failed to add reaction to activity list message! " + err.Error())
			sendError(s, m, "activity", Discord)
			return
		}
		err = s.MessageReactionAdd(m.ChannelID, message.ID, "▶️")
		if err != nil {
			logError("Failed to add reaction to activity list message! " + err.Error())
			sendError(s, m, "activity", Discord)
			return
		}
	case "autokick":
		if !userHasValidPermissions(s, m, discordgo.PermissionManageServer) {
			logWarning("User without appropriate permissions tried to mess with autokick")
			sendError(s, m, "activity", Permissions)
			return
		}
		// set autokick day check
		if len(command) != 3 && len(command) != 2 {
			sendError(s, m, "activity", Syntax)
			return
		}

		if len(command) == 2 {
			results, err := connection_pool.Query(fmt.Sprintf("SELECT * FROM %s WHERE (guild_id = ?);", autokickTable), m.GuildID)
			if err != nil {
				logError("Unable to read database for existing users in the guild! " + err.Error())
				sendError(s, m, "activity", Database)
				return
			}
			defer results.Close()

			autokickEnabled := false
			for results.Next() {
				autokickEnabled = true
				// send message
				var autokickData AutoKickData
				err := results.Scan(&autokickData.GuildID, &autokickData.DaysUntilKick)
				if err != nil {
					logError("Unable to parse database information! Aborting. " + err.Error())
					sendError(s, m, "activity", Database)
					return
				}
				attemptSendMsg(s, m, fmt.Sprintf("Current set to autokick users after %d days of inactivity.", autokickData.DaysUntilKick))
			}

			if !autokickEnabled {
				attemptSendMsg(s, m, "Autokick is currently disabled for the server.")
			}
			return
		}

		daysOfInactivity, err := strconv.Atoi(command[2])
		if err != nil {
			logError("Failed to convert string passed in to a number! " + err.Error())
			sendError(s, m, "activity", Syntax)
			return
		}
		logInfo(strconv.Itoa(daysOfInactivity))

		if daysOfInactivity < 1 {
			// remove autokick time from table
			if attemptQuery(fmt.Sprintf("DELETE FROM %s WHERE (guild_id = ?);", autokickTable), "Deactivated auto-kick", "Failed to deactivate auto-kick", m.GuildID) {
				sendSuccess(s, m, "")
			} else {
				logWarning("Failed to delete autokick entry! Is the connection still available?")
				sendError(s, m, "activity", Database)
			}
		} else {
			// set autokick day count
			if !attemptQuery(fmt.Sprintf("DELETE FROM %s WHERE (guild_id = ?);", autokickTable), "Deleted old auto-kick delay", "Failed to delete old auto-kick delay", m.GuildID) {
				logWarning("Failed to delete old autokick entry! Is the connection still available?")
				sendError(s, m, "activity", Database)
				return
			}

			if attemptQuery(fmt.Sprintf("INSERT INTO %s (guild_id, days_until_kick) VALUES (?, ?);", autokickTable), "Inserted new entry", "Failed to insert new entry", m.GuildID, daysOfInactivity) {
				logSuccess("Updated server in autokick table and notified user")
				sendSuccess(s, m, "")
			} else {
				logWarning("Failed to insert new autokick entry! Is the connection still available?")
				sendError(s, m, "activity", Database)
			}
		}
	case "whitelist":
		if !userHasValidPermissions(s, m, discordgo.PermissionKickMembers) {
			logWarning("User attempted to use whitelist without proper permissions")
			sendError(s, m, "activity", Permissions)
			return
		}
		// ensure user is valid, then toggle that user in memberActivity
		if len(command) != 4 {
			sendError(s, m, "activity", Syntax)
			return
		}

		userID := stripUserID(command[2])
		matched, _ := regexp.MatchString(`^[0-9]{18}$`, userID)
		if !matched {
			logWarning("User ID '" + userID + "' is invalid")
			sendError(s, m, "activity", Syntax)
			return
		}

		// update user's whitelist state
		if command[3] != "true" && command[3] != "false" {
			logWarning("User did not set 'true' or 'false' for the user's whitelist state")
			sendError(s, m, "activity", Syntax)
			return
		}

		tinyint := 0
		if command[3] == "true" {
			tinyint = 1
		}

		if attemptQuery(fmt.Sprintf("UPDATE %s SET whitelist = ? WHERE (guild_id = ? AND member_id = ?);", activityTable),
			"Updated whitelist with new user",
			"Failed to update whitelist with new user",
			tinyint, m.GuildID, userID) {
			if command[3] != "true" && command[3] != "false" {
				sendError(s, m, "activity", Syntax)
				return
			}

			sendSuccess(s, m, "")
		} else {
			logWarning("Query failed")
			sendError(s, m, "activity", Database)
		}
	default:
		sendError(s, m, "activity", Syntax)
	}
}

/**
When the shrine end timestamp has been passed, construct a message and post it to the designated
autoshrine channel.
*/
func handleShrineUpdate(s *discordgo.Session) {
	query, err := connection_pool.Query(fmt.Sprintf("SELECT * FROM %s", autoshrineTable))
	if err != nil {
		logError("SELECT query error, so stopping execution: " + err.Error())
		return
	}
	defer query.Close()

	// scrape latest shrine
	shrine := scrapeShrine()
	perksStr := ""
	for i := 0; i < len(shrine.Perks); i++ {
		perksStr += fmt.Sprintf("[%s](%s)\n", shrine.Perks[i].Id, shrine.Perks[i].Url)
	}

	for query.Next() {
		// write and send embed
		var autoshrineData ModLogData
		err = query.Scan(&autoshrineData.GuildID, &autoshrineData.ChannelID)
		if err != nil {
			logError("Unable to parse database information! Aborting. " + err.Error())
			return
		}
		logInfo("Sending shrine data to " + autoshrineData.GuildID + " " + autoshrineData.ChannelID)

		// images
		var image_embeds []*discordgo.MessageEmbed
		for i := 0; i < 4; i++ {

			var image discordgo.MessageEmbedImage
			image.URL = fmt.Sprintf("https://raw.githubusercontent.com/cazwacki/cazwacki.github.io/master/public/dbd/%s", shrine.Perks[i].Img_Url)
			image_embeds = append(image_embeds, &discordgo.MessageEmbed{
				Image: &image,
				URL:   "https://google.com",
			})
		}
		_, err = s.ChannelMessageSendEmbeds(autoshrineData.ChannelID, image_embeds)
		if err != nil {
			logError("Failed to send shrine image embed! " + err.Error())
			return
		}

		// construct embed response
		var embed discordgo.MessageEmbed
		embed.Type = "rich"
		embed.Title = "Shrine Has Been Updated!"
		var fields []*discordgo.MessageEmbedField
		for i := 0; i < len(shrine.Perks); i++ {
			info := fmt.Sprintf("[Wiki Link](%s)\n", shrine.Perks[i].Url) + shrine.Perks[i].Description
			fields = append(fields, createField(shrine.Perks[i].Id, info, false))
		}
		embed.Fields = fields
		var thumbnail discordgo.MessageEmbedThumbnail
		thumbnail.URL = "https://gamepedia.cursecdn.com/deadbydaylight_gamepedia_en/thumb/1/14/IconHelp_shrineOfSecrets.png/32px-IconHelp_shrineOfSecrets.png"
		embed.Thumbnail = &thumbnail

		// send response
		_, err = s.ChannelMessageSendEmbed(autoshrineData.ChannelID, &embed)
		if err != nil {
			logError("Failed to send embed! " + err.Error())
			return
		}
	}
}

/**
Switches the channel that the tweet monitoring system will output to.
**/
func handleAutoshrine(s *discordgo.Session, m *discordgo.MessageCreate, command []string) {
	if !userHasValidPermissions(s, m, discordgo.PermissionManageServer) {
		sendError(s, m, "autoshrine", Permissions)
		return
	}

	if len(command) != 3 && len(command) != 2 {
		sendError(s, m, "autoshrine", Syntax)
		return
	}

	switch command[1] {
	case "set":
		// create or update entry in database
		channel := strings.ReplaceAll(command[2], "<#", "")
		channel = strings.ReplaceAll(channel, ">", "")
		matched, _ := regexp.MatchString(`^[0-9]{18}$`, channel)
		if !matched {
			logInfo("User did not specify channel correctly")
			sendError(s, m, "autoshrine", Syntax)
			return
		}

		// remove old channel if it exists
		if !attemptQuery(fmt.Sprintf("DELETE FROM %s WHERE (guild_id = ?);",
			autoshrineTable),
			"Removed old autoshrine channel",
			"Couldn't remove old autoshrine channel! Is the connection still available?",
			m.GuildID) {
			sendError(s, m, "autoshrine", Database)
			return
		}

		// add new channel
		if attemptQuery(fmt.Sprintf("INSERT INTO %s (guild_id, channel_id) VALUES (?, ?);", autoshrineTable),
			"Added new autoshrine channel",
			"Couldn't add new autoshrine channel! Is the connection still available?",
			m.GuildID, channel) {
			logSuccess("Set new autoshrine channel")
			sendSuccess(s, m, "")
			return
		} else {
			sendError(s, m, "autoshrine", Database)
		}
	case "reset":
		// remove old channel if it exists
		if attemptQuery(fmt.Sprintf("DELETE FROM %s WHERE (guild_id = ?);", autoshrineTable),
			"Removed old autoshrine channel",
			"Couldn't remove old autoshrine channel! Is the connection still available?",
			m.GuildID) {
			logSuccess("Reset autoshrine channel")
			sendSuccess(s, m, "")
			return
		} else {
			sendError(s, m, "autoshrine", Database)
		}
	}
}

/**
Returns the users from the database who have been inactive for the requested number of days or more.
*/
func getInactiveUsers(s *discordgo.Session, m *discordgo.MessageCreate, command []string) []MemberActivity {
	var inactiveUsers []MemberActivity
	// fetch all users in this guild, then filter to users who have been inactive more than <number> days

	results, err := connection_pool.Query(fmt.Sprintf("SELECT * FROM %s WHERE (guild_id = ?);", activityTable), m.GuildID)
	if err != nil {
		logError("Unable to read database for existing users in the guild! " + err.Error())
		sendError(s, m, "activity", Database)
		return inactiveUsers
	}
	defer results.Close()

	// loop through members in the database and store them in an array
	dateFormat := "2006-01-02 15:04:05.999999999 -0700 MST"

	for results.Next() {
		var memberActivity MemberActivity
		err = results.Scan(&memberActivity.ID, &memberActivity.GuildID, &memberActivity.MemberID, &memberActivity.MemberName, &memberActivity.LastActive, &memberActivity.Description, &memberActivity.Whitelisted)
		if err != nil {
			logError("Unable to parse database information! Aborting. " + err.Error())
			sendError(s, m, "activity", Database)
			return inactiveUsers
		}

		daysInactive, err := strconv.Atoi(command[2])
		if err != nil {
			sendError(s, m, "activity", Syntax)
			return inactiveUsers
		}

		if daysInactive < 1 {
			inactiveUsers = append(inactiveUsers, memberActivity)
		} else {
			// calculate difference between time.Now() and the provided timestamp
			lastActive, err := time.Parse(dateFormat, strings.Split(memberActivity.LastActive, " m=")[0])
			if err != nil {
				logError("Unable to parse database timestamps! Aborting. " + err.Error())
				sendError(s, m, "activity", Database)
				return inactiveUsers
			}
			lastActive = lastActive.AddDate(0, 0, daysInactive)
			if lastActive.Before(time.Now()) {
				inactiveUsers = append(inactiveUsers, memberActivity)
			}
		}
	}

	return inactiveUsers
}
