package main

import (
	"database/sql"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
	_ "github.com/go-sql-driver/mysql"
)

var dbUsername string
var dbPassword string
var db string
var dbTable string

type MemberActivity struct {
	ID          int    `json:"entry"`
	GuildID     string `json:"guild_id"`
	MemberID    string `json:"member_id"`
	MemberName  string `json:"member_name"`
	LastActive  string `json:"last_active"`
	Description string `json:"description"`
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

	db, err := sql.Open("mysql", fmt.Sprintf("%s:%s@tcp(192.168.0.117:3306)/%s", dbUsername, dbPassword, db))
	defer db.Close()

	if err != nil {
		fmt.Println("Unable to open DB connection! " + err.Error())
		return
	}

	if len(description) > 80 {
		description = description[0:80]
	}

	if newUser {
		// INSERT INTO table (guild_id, member_id, last_active, description) VALUES (guildID, userID, time, description)
		query, err := db.Query("INSERT INTO " + dbTable + " (guild_id, member_id, member_name, last_active, description) VALUES ('" + guildID + "', '" + user.ID + "', '" + user.Username + "#" + user.Discriminator + "', '" + time + "', '" + description + "');")
		defer query.Close()
		if err != nil {
			fmt.Println("Unable to insert new user! " + err.Error())
			return
		}
	} else {
		// UPDATE table SET (last_active = time, description = description) WHERE (guild_id = guildID AND member_id = userID)
		query, err := db.Query("UPDATE " + dbTable + " SET last_active = '" + time + "', description = '" + description + "', member_name = '" + user.Username + "#" + user.Discriminator + "' WHERE (guild_id = '" + guildID + "' AND member_id = '" + user.ID + "');")
		defer query.Close()
		if err != nil {
			fmt.Println("Unable to update user's activity! " + err.Error())
			return
		}
	}

}

// removes the user's row when they leave the server.
func removeUser(guildID string, userID string) {
	db, err := sql.Open("mysql", fmt.Sprintf("%s:%s@tcp(192.168.0.117:3306)/%s", dbUsername, dbPassword, db))
	defer db.Close()

	if err != nil {
		fmt.Println("Unable to open DB connection! " + err.Error())
		return
	}

	query, err := db.Query("DELETE FROM " + dbTable + " WHERE (guild_id = '" + guildID + "' AND member_id = '" + userID + "');")
	defer query.Close()
	if err != nil {
		fmt.Println("Unable to delete user's activity! " + err.Error())
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
			fmt.Println("Unable to scan the full guild... " + err.Error())
			return 0
		}
		if len(nextMembers) < 1000 {
			scanning = false
		}
		memberList = append(memberList, nextMembers...)
		after = memberList[len(memberList)-1].User.ID
	}

	db, err := sql.Open("mysql", fmt.Sprintf("%s:%s@tcp(192.168.0.117:3306)/%s", dbUsername, dbPassword, db))
	defer db.Close()

	if err != nil {
		fmt.Println("Unable to open DB connection! " + err.Error())
		return 0
	}

	results, err := db.Query("SELECT * FROM " + dbTable + " WHERE (guild_id = '" + guildID + "')")
	defer results.Close()

	if err != nil {
		fmt.Println("Unable to read database for existing users in the guild! " + err.Error())
		return 0
	}

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
	db, err := sql.Open("mysql", fmt.Sprintf("%s:%s@tcp(192.168.0.117:3306)/%s", dbUsername, dbPassword, db))
	defer db.Close()

	query, err := db.Query("DELETE FROM " + dbTable + " WHERE (guild_id = '" + guildID + "')")
	defer query.Close()
	if err != nil {
		fmt.Println("Unable to delete guild from database! " + err.Error())
		return
	}

	if err != nil {
		fmt.Println("Unable to open DB connection! " + err.Error())
		return
	}
}

/****
COMMANDS
****/
func activity(s *discordgo.Session, m *discordgo.MessageCreate, command []string) {
	if len(command) < 2 && len(command) > 3 {
		s.ChannelMessageSend(m.ChannelID, "Usages: ```~activity rescan\n~activity list <number>```")
		return
	}

	switch command[1] {
	case "rescan":
		if len(command) != 2 {
			s.ChannelMessageSend(m.ChannelID, "Usages: ```~activity rescan\n~activity list <number>```")
			return
		}
		membersAdded := logNewGuild(s, m.GuildID)
		s.ChannelMessageSend(m.ChannelID, "Added "+strconv.Itoa(membersAdded)+" members to the database!")
	case "user":
		fmt.Println(command)
		regex := regexp.MustCompile(`^\<\@\!?[0-9]+\>$`)
		if regex.MatchString(command[2]) {
			userID := strings.TrimSuffix(command[2], ">")
			userID = strings.TrimPrefix(userID, "<@")
			userID = strings.TrimPrefix(userID, "!") // this means the user has a nickname

			// parse userID, get it from the db, present info
			db, err := sql.Open("mysql", fmt.Sprintf("%s:%s@tcp(192.168.0.117:3306)/%s", dbUsername, dbPassword, db))
			defer db.Close()

			if err != nil {
				fmt.Println("Unable to open DB connection! " + err.Error())
				return
			}

			query, err := db.Query("SELECT * FROM " + dbTable + " WHERE (guild_id = '" + m.GuildID + "' AND member_id = '" + userID + "');")
			defer query.Close()

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
		s.ChannelMessageSend(m.ChannelID, "Usages: ```~activity rescan\n~activity list <number>```")
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
		s.ChannelMessageSend(m.ChannelID, "Usages: ```~activity rescan\n~activity list <number>```")
		return inactiveUsers
	}

	db, err := sql.Open("mysql", fmt.Sprintf("%s:%s@tcp(192.168.0.117:3306)/%s", dbUsername, dbPassword, db))
	defer db.Close()

	if err != nil {
		fmt.Println("Unable to open DB connection! " + err.Error())
		return inactiveUsers
	}

	results, err := db.Query("SELECT * FROM " + dbTable + " WHERE (guild_id = '" + m.GuildID + "')")
	defer results.Close()

	if err != nil {
		fmt.Println("Unable to read database for existing users in the guild! " + err.Error())
		return inactiveUsers
	}

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
			s.ChannelMessageSend(m.ChannelID, "Usages: ```~activity rescan\n~activity list <number>```")
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
