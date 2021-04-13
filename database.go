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
var db string
var activityTable string
var leaderboardTable string
var joinLeaveTable string

type MemberActivity struct {
	ID          int    `json:"entry"`
	GuildID     string `json:"guild_id"`
	MemberID    string `json:"member_id"`
	MemberName  string `json:"member_name"`
	LastActive  string `json:"last_active"`
	Description string `json:"description"`
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
		insertSQL := fmt.Sprintf("INSERT INTO %s (guild_id, member_id, member_name, last_active, description) VALUES ('%s', '%s', '%s', '%s', '%s');",
			activityTable, guildID, user.ID, strings.ReplaceAll(user.Username, "'", "\\'")+"#"+user.Discriminator, time, description)
		queryWithoutResults(insertSQL, "Unable to insert new user!")
	} else {
		// UPDATE table SET (last_active = time, description = description) WHERE (guild_id = guildID AND member_id = userID)
		updateSQL := fmt.Sprintf("UPDATE %s SET last_active = '%s', description = '%s', member_name = '%s' WHERE (guild_id = '%s' AND member_id = '%s');",
			activityTable, time, description, strings.ReplaceAll(user.Username, "'", "\\'")+"#"+user.Discriminator, guildID, user.ID)
		queryWithoutResults(updateSQL, "Unable to update user's activity!")
	}

}

// removes the user's row when they leave the server.
func removeUser(guildID string, userID string) {
	deleteSQL := fmt.Sprintf("DELETE FROM %s WHERE (guild_id = '%s' AND member_id = '%s');", activityTable, guildID, userID)
	queryWithoutResults(deleteSQL, "Unable to delete user's activity!")
}

// sends the guild's join/leave message when a user enters/leaves the server.
func joinLeaveMessage(s *discordgo.Session, guildID string, user *discordgo.User, messageType string) {
	selectSQL := fmt.Sprintf("SELECT * FROM %s WHERE (guild_id = '%s' AND message_type = '%s');", joinLeaveTable, guildID, messageType)
	query, err := connection_pool.Query(selectSQL)
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

	results, err := connection_pool.Query("SELECT * FROM " + activityTable + " WHERE (guild_id = '" + guildID + "')")
	if err != nil {
		logError("Unable to read database for existing users in the guild! " + err.Error())
		return 0
	}
	defer results.Close()

	// loop through members in the database and store them in an array
	var memberActivities []MemberActivity
	for results.Next() {
		var memberActivity MemberActivity
		err = results.Scan(&memberActivity.ID, &memberActivity.GuildID, &memberActivity.MemberID, &memberActivity.MemberName, &memberActivity.LastActive, &memberActivity.Description)
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
	deleteSQL := fmt.Sprintf("DELETE FROM %s WHERE (guild_id = '%s');", activityTable, guildID)
	queryWithoutResults(deleteSQL, "Unable to delete guild from database!")
}

// awards a user points for the guild's leaderboard based on the word count formula.
func awardPoints(guildID string, user *discordgo.User, currentTime string, message string) {
	if user.Bot {
		return
	}

	// 1. calculate points from sentence
	wordCount := len(strings.Split(message, " "))

	pointsToAward := int(math.Floor(math.Pow(float64(wordCount), float64(1)/3)*10 - 10))

	selectSQL := fmt.Sprintf("SELECT * FROM %s WHERE (guild_id = '%s' AND member_id = '%s');", leaderboardTable, guildID, user.ID)
	query, err := connection_pool.Query(selectSQL)
	if err != nil {
		logError("SELECT query error, but not stopping execution: " + err.Error())
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
				updateSQL := fmt.Sprintf("UPDATE %s SET last_awarded = '%s', points = '%d', member_name = '%s' WHERE (guild_id = '%s' AND member_id = '%s');",
					leaderboardTable, currentTime, newScore, strings.ReplaceAll(user.Username, "'", "\\'")+"#"+user.Discriminator, guildID, user.ID)
				queryWithoutResults(updateSQL, "Unable to update member's points in database!")
			}
		}
	}

	if !foundUser {
		insertSQL := fmt.Sprintf("INSERT INTO %s (guild_id, member_id, member_name, points, last_awarded) VALUES ('%s', '%s', '%s', '%d', '%s');",
			leaderboardTable, guildID, user.ID, strings.ReplaceAll(user.Username, "'", "\\'")+"#"+user.Discriminator, pointsToAward, currentTime)
		queryWithoutResults(insertSQL, "awardPoints(): Unable to insert new user!")
		return
	}
}

// helper function for queries we don't need the results for.
func queryWithoutResults(sql string, errMessage string) {
	query, err := connection_pool.Query(sql)
	if err != nil {
		logError(errMessage + " " + err.Error())
	}
	defer query.Close()
}

/****
COMMANDS
****/
func greeter(s *discordgo.Session, m *discordgo.MessageCreate, command []string) {
	logInfo(strings.Join(command, " "))
	if !userHasValidPermissions(s, m, discordgo.PermissionManageServer) {
		_, err := s.ChannelMessageSend(m.ChannelID, "Sorry, you aren't allowed to manage this.")
		if err != nil {
			logError("Failed to send permissions message! " + err.Error())
		}
		return
	}
	if len(command) == 1 {
		_, err := s.ChannelMessageSend(m.ChannelID, "Use ~greeter help to learn more about this command.")
		if err != nil {
			logError("Failed to send usage message! " + err.Error())
		}
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
			return
		}
		logSuccess("Sent user help embed for greeter")
	case "status":
		selectSQL := fmt.Sprintf("SELECT * FROM %s WHERE (guild_id = '%s');", joinLeaveTable, m.GuildID)
		query, err := connection_pool.Query(selectSQL)
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
			_, err := s.ChannelMessageSend(m.ChannelID, "This server currently has no greeter messages!")
			if err != nil {
				logError("Failed to send 'no greeter messages' message! " + err.Error())
				return
			}
		}
		logSuccess("Sent greeter status to user")
	case "set":
		if len(command) < 5 {
			logInfo("User did not use enough arguments when calling greeter set")
			_, err := s.ChannelMessageSend(m.ChannelID, "Use ~greeter help to learn more about this command.")
			if err != nil {
				logError("Failed to send usage message! " + err.Error())
			}
			return
		}
		if command[2] != "join" && command[2] != "leave" {
			logInfo("User did not use 'join' or 'leave' when calling greeter set")
			_, err := s.ChannelMessageSend(m.ChannelID, "You must specify whether you are setting the join or leave message.")
			if err != nil {
				logError("Failed to send greeter set error message! " + err.Error())
			}
			return
		}

		channel := strings.ReplaceAll(command[3], "<#", "")
		channel = strings.ReplaceAll(channel, ">", "")
		matched, _ := regexp.MatchString(`^[0-9]{18}$`, channel)
		if !matched {
			logInfo("User did not specify channel correctly")
			_, err := s.ChannelMessageSend(m.ChannelID, "You must specify the channel correctly.")
			if err != nil {
				logError("Failed to send greeter set error message! " + err.Error())
			}
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
		deleteSQL := fmt.Sprintf("DELETE FROM %s WHERE (guild_id = '%s' AND message_type = '%s');", joinLeaveTable, m.GuildID, command[2])
		queryWithoutResults(deleteSQL, fmt.Sprintf("Unable to delete old %s message!", command[2]))

		// add new message
		insertSQL := fmt.Sprintf("INSERT INTO %s (guild_id, channel_id, message_type, image_link, message) VALUES ('%s', '%s', '%s', '%s', '%s');",
			joinLeaveTable, m.GuildID, channel, command[2], imageURL, message)
		queryWithoutResults(insertSQL, fmt.Sprintf("Unable to set new %s message!", command[2]))

		_, err := s.ChannelMessageSend(m.ChannelID, fmt.Sprintf("Set the new message when user %ss! Use `~greeter status` to check your messages for this server.", command[2]))
		if err != nil {
			logError("Failed to send greeter set success message! " + err.Error())
			return
		}
		logSuccess("Set new greeter message")
	case "reset":
		if len(command) != 3 {
			logInfo("User did not pass in the correct number of arguments for greeter reset")
			_, err := s.ChannelMessageSend(m.ChannelID, "Use ~greeter help to learn more about this command.")
			if err != nil {
				logError("Failed to send greeter usage message! " + err.Error())
			}
			return
		}
		if command[2] != "join" && command[2] != "leave" {
			logInfo("User did not specify whether the join or leave message was being reset")
			_, err := s.ChannelMessageSend(m.ChannelID, "You must specify whether you are resetting the join or leave message.")
			if err != nil {
				logError("Failed to send reset misuse message! " + err.Error())
			}
			return
		}
		deleteSQL := fmt.Sprintf("DELETE FROM %s WHERE (guild_id = '%s' AND message_type = '%s');", joinLeaveTable, m.GuildID, command[2])
		queryWithoutResults(deleteSQL, "Unable to delete the message!")
		_, err := s.ChannelMessageSend(m.ChannelID, fmt.Sprintf("Removed message when user %ss, if there was an existing message.", command[2]))
		if err != nil {
			logError("Failed to send greeter reset success message! " + err.Error())
		}
		logSuccess("Notified user that greeter message is removed")
	default:
		_, err := s.ChannelMessageSend(m.ChannelID, "Use ~greeter help to learn more about this command.")
		if err != nil {
			logError("Failed to send greeter usage message! " + err.Error())
			return
		}
		logSuccess("Sent usage message to user")
	}
}

func leaderboard(s *discordgo.Session, m *discordgo.MessageCreate, command []string) {
	logInfo(strings.Join(command, " "))
	if len(command) > 1 {
		logInfo("User passed in incorrect number of arguments")
		_, err := s.ChannelMessageSend(m.ChannelID, "Usages: ```~leaderboard```")
		if err != nil {
			logError("Failed to send usage message to channel! " + err.Error())
		}
		return
	}

	if len(command) == 1 {
		// generate leaderboard of top 10 users with corresponding points, with user's score at the bottom

		// 1. Get all members of the guild the command was invoked in and sort by points
		selectSQL := fmt.Sprintf("SELECT * FROM %s WHERE (guild_id = '%s');", leaderboardTable, m.GuildID)
		results, err := connection_pool.Query(selectSQL)
		if err != nil {
			_, msgErr := s.ChannelMessageSend(m.ChannelID, "Unable to read database for existing users in the guild! "+err.Error())
			if msgErr != nil {
				logError("Failed to send database error message to channel! " + msgErr.Error())
			}
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
		_, err = s.ChannelMessageSend(m.ChannelID, message)
		if err != nil {
			logError("Failed to send leaderboard message! " + err.Error())
			return
		}
		logSuccess("Sent leaderboard message")
	}
}

func activity(s *discordgo.Session, m *discordgo.MessageCreate, command []string) {
	logInfo(strings.Join(command, " "))
	if len(command) < 2 || len(command) > 3 {
		_, err := s.ChannelMessageSend(m.ChannelID, "Usages: ```~activity rescan\n~activity list <number>\n~activity user <@user>```")
		if err != nil {
			logError("Failed to send usage message! " + err.Error())
		}
		return
	}

	switch command[1] {
	case "rescan":
		if len(command) != 2 {
			_, err := s.ChannelMessageSend(m.ChannelID, "Usages: ```~activity rescan\n~activity list <number>\n~activity user <@user>```")
			if err != nil {
				logError("Failed to send usage message! " + err.Error())
			}
			return
		}
		membersAdded := logNewGuild(s, m.GuildID)
		_, err := s.ChannelMessageSend(m.ChannelID, "Added "+strconv.Itoa(membersAdded)+" members to the database!")
		if err != nil {
			logError("Failed to send rescan result message! " + err.Error())
			return
		}
		logSuccess("Found " + strconv.Itoa(membersAdded) + " new users, added them to database, and sent rescan result message")
	case "user":
		regex := regexp.MustCompile(`^\<\@\!?[0-9]+\>$`)
		if regex.MatchString(command[2]) {
			userID := stripUserID(command[2])

			// parse userID, get it from the db, present info
			selectSQL := fmt.Sprintf("SELECT * FROM %s WHERE (guild_id = '%s' AND member_id = '%s');", activityTable, m.GuildID, userID)
			query, err := connection_pool.Query(selectSQL)
			defer query.Close()
			if err == sql.ErrNoRows {
				logWarning("User not found in the database. This usually should not happen.")
				_, msgErr := s.ChannelMessageSend(m.ChannelID, "This user isn't in our database... :frowning:")
				if msgErr != nil {
					logError("Failed to send 'failed to find member' message! " + err.Error())
					return
				}
				return
			} else {
				for query.Next() {
					var memberActivity MemberActivity
					err = query.Scan(&memberActivity.ID, &memberActivity.GuildID, &memberActivity.MemberID, &memberActivity.MemberName, &memberActivity.LastActive, &memberActivity.Description)
					if err != nil {
						logError("Unable to parse database information! Aborting. " + err.Error())
						return
					}
					dateFormat := "2006-01-02 15:04:05.999999999 -0700 MST"
					lastActive, err := time.Parse(dateFormat, strings.Split(memberActivity.LastActive, " m=")[0])
					if err != nil {
						logError("Unable to parse database timestamps! Aborting. " + err.Error())
						return
					}
					var embed discordgo.MessageEmbed
					embed.Type = "rich"
					embed.Title = memberActivity.MemberName
					embed.Description = "- " + lastActive.Format("01/02/2006 15:04:05") + "\n- " + memberActivity.Description

					member, err := s.GuildMember(m.GuildID, userID)
					if err != nil {
						logError("Couldn't pull member information from the session. " + err.Error())
						_, msgErr := s.ChannelMessageSend(m.ChannelID, "Couldn't get the user's guild info... :frowning:")
						if msgErr != nil {
							logError("Failed to send rescan result message! " + err.Error())
							return
						}
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
					logSuccess("Sent user activity message")
				}
			}
		}
	case "list":
		inactiveUsers := getInactiveUsers(s, m, command)
		daysOfInactivity, _ := strconv.Atoi(command[2])

		if len(inactiveUsers) == 0 {
			_, err := s.ChannelMessageSend(m.ChannelID, "No user has been inactive for "+strconv.Itoa(daysOfInactivity)+"+ days.")
			if err != nil {
				logError("Failed to send 'no users inactive' message! " + err.Error())
				return
			}
			logSuccess("Returned that there were no inactive users")
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
				return
			}
			contents = append(contents, createField(inactiveUsers[i].MemberName, "- "+lastActive.Format("01/02/2006 15:04:05")+"\n- "+inactiveUsers[i].Description, false))
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
			return
		}
		err = s.MessageReactionAdd(m.ChannelID, message.ID, "▶️")
		if err != nil {
			logError("Failed to add reaction to activity list message! " + err.Error())
			return
		}
		logSuccess("Returned interactable activity list")
	default:
		_, err := s.ChannelMessageSend(m.ChannelID, "Usages: ```~activity rescan\n~activity list <number>\n~activity user <@user>```")
		if err != nil {
			logError("Failed to send activity usage message! " + err.Error())
			return
		}
		return
	}
}

/**
Returns the users from the database who have been inactive for the requested number of days or more.
*/
func getInactiveUsers(s *discordgo.Session, m *discordgo.MessageCreate, command []string) []MemberActivity {
	var inactiveUsers []MemberActivity
	// fetch all users in this guild, then filter to users who have been inactive more than <number> days
	if len(command) != 3 {
		_, err := s.ChannelMessageSend(m.ChannelID, "Usages: ```~activity rescan\n~activity list <number>\n~activity user <@user>```")
		if err != nil {
			logError("Failed to send usage message! " + err.Error())
		}
		return inactiveUsers
	}

	selectSQL := fmt.Sprintf("SELECT * FROM %s WHERE (guild_id = '%s');", activityTable, m.GuildID)
	results, err := connection_pool.Query(selectSQL)
	if err != nil {
		logError("Unable to read database for existing users in the guild! " + err.Error())
		return inactiveUsers
	}
	defer results.Close()

	// loop through members in the database and store them in an array
	dateFormat := "2006-01-02 15:04:05.999999999 -0700 MST"

	for results.Next() {
		var memberActivity MemberActivity
		err = results.Scan(&memberActivity.ID, &memberActivity.GuildID, &memberActivity.MemberID, &memberActivity.MemberName, &memberActivity.LastActive, &memberActivity.Description)
		if err != nil {
			logError("Unable to parse database information! Aborting. " + err.Error())
			return inactiveUsers
		}
		daysInactive, err := strconv.Atoi(command[2])
		if err != nil {
			_, msgErr := s.ChannelMessageSend(m.ChannelID, "Usages: ```~activity rescan\n~activity list <number>\n~activity user <@user>```")
			if msgErr != nil {
				logError("Failed to send usage message! " + err.Error())
			}
			return inactiveUsers
		}

		if daysInactive < 1 {
			inactiveUsers = append(inactiveUsers, memberActivity)
		} else {
			// calculate difference between time.Now() and the provided timestamp
			lastActive, err := time.Parse(dateFormat, strings.Split(memberActivity.LastActive, " m=")[0])
			if err != nil {
				logError("Unable to parse database timestamps! Aborting. " + err.Error())
				return inactiveUsers
			}
			lastActive = lastActive.AddDate(0, 0, daysInactive)
			if lastActive.Before(time.Now()) {
				inactiveUsers = append(inactiveUsers, memberActivity)
			}
		}
	}
	logSuccess("Searched for inactive users without any errors")
	return inactiveUsers
}
