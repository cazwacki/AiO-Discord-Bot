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

// loads the provided guild's members into the database.
func logNewGuild(s *discordgo.Session, guildID string) int {

	// loop through members and populate our list of users in the guild
	var memberList []*discordgo.Member
	after := ""
	scanning := true
	for scanning {
		nextMembers, err := s.GuildMembers(guildID, after, 500)
		if err != nil {
			fmt.Println("Unable to scan the full guild... " + err.Error())
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
		fmt.Println("Unable to read database for existing users in the guild! " + err.Error())
		return 0
	}
	defer results.Close()

	// loop through members in the database and store them in an array
	var memberActivities []MemberActivity
	for results.Next() {
		var memberActivity MemberActivity
		err = results.Scan(&memberActivity.ID, &memberActivity.GuildID, &memberActivity.MemberID, &memberActivity.MemberName, &memberActivity.LastActive, &memberActivity.Description)
		if err != nil {
			fmt.Println("Unable to parse database information! Aborting. " + err.Error())
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
		fmt.Println("Error with SELECT query: " + err.Error())
	}
	defer query.Close()

	foundUser := false
	for query.Next() {
		foundUser = true
		var leaderboardEntry LeaderboardEntry
		err = query.Scan(&leaderboardEntry.ID, &leaderboardEntry.GuildID, &leaderboardEntry.MemberID, &leaderboardEntry.MemberID, &leaderboardEntry.Points, &leaderboardEntry.LastAwarded)
		if err != nil {
			fmt.Println("Unable to parse database information! Aborting. " + err.Error())
			return
		} else {
			dateFormat := "2006-01-02 15:04:05.999999999 -0700 MST"
			lastAwarded, err := time.Parse(dateFormat, strings.Split(leaderboardEntry.LastAwarded, " m=")[0])
			if err != nil {
				fmt.Println("Unable to parse database timestamps! Aborting. " + err.Error())
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
		fmt.Println(errMessage + " " + err.Error())
	}
	defer query.Close()
}

/****
COMMANDS
****/
func leaderboard(s *discordgo.Session, m *discordgo.MessageCreate, command []string) {
	if len(command) > 1 {
		s.ChannelMessageSend(m.ChannelID, "Usages: ```~leaderboard```")
		return
	}

	if len(command) == 1 {
		// generate leaderboard of top 10 users with corresponding points, with user's score at the bottom

		// 1. Get all members of the guild the command was invoked in and sort by points
		selectSQL := fmt.Sprintf("SELECT * FROM %s WHERE (guild_id = '%s');", leaderboardTable, m.GuildID)
		results, err := connection_pool.Query(selectSQL)
		if err != nil {
			s.ChannelMessageSend(m.ChannelID, "Unable to read database for existing users in the guild! "+err.Error())
			return
		}
		defer results.Close()

		// create array of users
		var leaderboardEntries []LeaderboardEntry
		for results.Next() {
			var entry LeaderboardEntry
			err = results.Scan(&entry.ID, &entry.GuildID, &entry.MemberID, &entry.MemberName, &entry.Points, &entry.LastAwarded)
			if err != nil {
				fmt.Println("Unable to parse database information! Aborting. " + err.Error())
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
		s.ChannelMessageSend(m.ChannelID, message)
	}
}

func activity(s *discordgo.Session, m *discordgo.MessageCreate, command []string) {
	if len(command) < 2 || len(command) > 3 {
		s.ChannelMessageSend(m.ChannelID, "Usages: ```~activity rescan\n~activity list <number>\n~activity user <@user>```")
		return
	}

	switch command[1] {
	case "rescan":
		if len(command) != 2 {
			s.ChannelMessageSend(m.ChannelID, "Usages: ```~activity rescan\n~activity list <number>\n~activity user <@user>```")
			return
		}
		membersAdded := logNewGuild(s, m.GuildID)
		s.ChannelMessageSend(m.ChannelID, "Added "+strconv.Itoa(membersAdded)+" members to the database!")
	case "user":
		fmt.Println(command)
		regex := regexp.MustCompile(`^\<\@\!?[0-9]+\>$`)
		if regex.MatchString(command[2]) {
			userID := stripUserID(command[2])

			// parse userID, get it from the db, present info
			selectSQL := fmt.Sprintf("SELECT * FROM %s WHERE (guild_id = '%s' AND member_id = '%s');", activityTable, m.GuildID, userID)
			query, err := connection_pool.Query(selectSQL)
			defer query.Close()
			if err == sql.ErrNoRows {
				s.ChannelMessageSend(m.ChannelID, "This user isn't in our database... :frowning:")
				return
			} else {
				for query.Next() {
					var memberActivity MemberActivity
					err = query.Scan(&memberActivity.ID, &memberActivity.GuildID, &memberActivity.MemberID, &memberActivity.MemberName, &memberActivity.LastActive, &memberActivity.Description)
					if err != nil {
						fmt.Println("Unable to parse database information! Aborting. " + err.Error())
						return
					}
					dateFormat := "2006-01-02 15:04:05.999999999 -0700 MST"
					lastActive, err := time.Parse(dateFormat, strings.Split(memberActivity.LastActive, " m=")[0])
					if err != nil {
						fmt.Println("Unable to parse database timestamps! Aborting. " + err.Error())
						return
					}
					var embed discordgo.MessageEmbed
					embed.Type = "rich"
					embed.Title = memberActivity.MemberName
					embed.Description = "- " + lastActive.Format("01/02/2006 15:04:05") + "\n- " + memberActivity.Description

					member, err := s.GuildMember(m.GuildID, userID)
					if err != nil {
						s.ChannelMessageSend(m.ChannelID, "Couldn't get the user's guild info... :frowning:")
						return
					}
					var thumbnail discordgo.MessageEmbedThumbnail
					thumbnail.URL = member.User.AvatarURL("")
					embed.Thumbnail = &thumbnail

					s.ChannelMessageSendEmbed(m.ChannelID, &embed)
				}
			}
		}
	case "list":
		inactiveUsers := getInactiveUsers(s, m, command)
		daysOfInactivity, _ := strconv.Atoi(command[2])

		if len(inactiveUsers) == 0 {
			s.ChannelMessageSend(m.ChannelID, "No user has been inactive for "+strconv.Itoa(daysOfInactivity)+"+ days.")
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
				fmt.Println("Unable to parse database timestamps! Aborting. " + err.Error())
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

		footer.Text = fmt.Sprintf("Page 1 of %d", pageCount)
		embed.Footer = &footer
		message, _ := s.ChannelMessageSendEmbed(m.ChannelID, &embed)

		newSet.Message = message
		go appendToGlobalInactiveSet(s, newSet)

		s.MessageReactionAdd(m.ChannelID, message.ID, "◀️")
		s.MessageReactionAdd(m.ChannelID, message.ID, "▶️")
	default:
		// something about usage
		s.ChannelMessageSend(m.ChannelID, "Usages: ```~activity rescan\n~activity list <number>\n~activity user <@user>```")
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
		s.ChannelMessageSend(m.ChannelID, "Usages: ```~activity rescan\n~activity list <number>\n~activity user <@user>```")
		return inactiveUsers
	}

	selectSQL := fmt.Sprintf("SELECT * FROM %s WHERE (guild_id = '%s');", activityTable, m.GuildID)
	results, err := connection_pool.Query(selectSQL)
	if err != nil {
		fmt.Println("Unable to read database for existing users in the guild! " + err.Error())
		return inactiveUsers
	}
	defer results.Close()

	// loop through members in the database and store them in an array
	dateFormat := "2006-01-02 15:04:05.999999999 -0700 MST"

	for results.Next() {
		var memberActivity MemberActivity
		err = results.Scan(&memberActivity.ID, &memberActivity.GuildID, &memberActivity.MemberID, &memberActivity.MemberName, &memberActivity.LastActive, &memberActivity.Description)
		if err != nil {
			fmt.Println("Unable to parse database information! Aborting. " + err.Error())
			return inactiveUsers
		}
		daysInactive, err := strconv.Atoi(command[2])
		if err != nil {
			s.ChannelMessageSend(m.ChannelID, "Usages: ```~activity rescan\n~activity list <number>\n~activity user <@user>```")
			return inactiveUsers
		}

		if daysInactive < 1 {
			inactiveUsers = append(inactiveUsers, memberActivity)
		} else {
			// calculate difference between time.Now() and the provided timestamp
			lastActive, err := time.Parse(dateFormat, strings.Split(memberActivity.LastActive, " m=")[0])
			if err != nil {
				fmt.Println("Unable to parse database timestamps! Aborting. " + err.Error())
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
