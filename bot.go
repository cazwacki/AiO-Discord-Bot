package main

import (
	"database/sql"
	"fmt"
	"math/rand"
	"os"
	"os/signal"
	"regexp"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/bwmarrin/discordgo"
)

var start time.Time
var prodMode bool
var globalImageSet []*ImageSet
var globalInactiveSet []*InactiveSet
var prefix string
var commandList map[string]command
var debug bool

type handler func(*discordgo.Session, *discordgo.MessageCreate, []string)

type command struct {
	handle handler
	usage  string
}

func appendToGlobalImageSet(s *discordgo.Session, newset ImageSet) {
	globalImageSet = append(globalImageSet, &newset)
	logInfo("Global Image Set:")
	logInfo(fmt.Sprintf("%+v\n", globalImageSet))

	time.Sleep(30 * time.Minute)

	for i, set := range globalImageSet {
		if &newset == set {
			tmpSet := globalImageSet[0]
			globalImageSet[0] = globalImageSet[i]
			globalImageSet[i] = tmpSet
			globalImageSet = globalImageSet[1:]
			err := s.MessageReactionsRemoveAll(newset.Message.ChannelID, newset.Message.ID)
			if err != nil {
				logError("Failed to remove reactions from the image embed! " + err.Error())
			}
		}
	}
}

func appendToGlobalInactiveSet(s *discordgo.Session, newset InactiveSet) {
	globalInactiveSet = append(globalInactiveSet, &newset)
	logInfo("Global Image Set:")
	logInfo(fmt.Sprintf("%+v\n", globalInactiveSet))

	time.Sleep(30 * time.Minute)

	for i, set := range globalInactiveSet {
		if &newset == set {
			tmpSet := globalInactiveSet[0]
			globalInactiveSet[0] = globalInactiveSet[i]
			globalInactiveSet[i] = tmpSet
			globalInactiveSet = globalInactiveSet[1:]
			err := s.MessageReactionsRemoveAll(newset.Message.ChannelID, newset.Message.ID)
			if err != nil {
				logError("Failed to remove reactions from the inactivity list! " + err.Error())
			}
		}
	}
}

/**
Initialize command information and prefix
*/
func initCommandInfo() {
	prefix = "~"
	commandList = map[string]command{
		"uptime":      {handleUptime, "~uptime"},
		"shutdown":    {handleShutdown, "~shutdown"},
		"invite":      {handleInvite, "~invite"},
		"profile":     {attemptProfile, "~profile"},
		"nick":        {handleNickname, "~nick @user (reason: optional)"},
		"kick":        {handleKick, "~kick @user (reason: optional)"},
		"ban":         {handleBan, "~ban @user (reason: optional)"},
		"mv":          {handleMove, "~mv <number> #channel"},
		"cp":          {handleCopy, "~cp <number> #channel"},
		"purge":       {handlePurge, "~purge <number>"},
		"define":      {handleDefine, "~define <word / phrase>"},
		"urban":       {handleUrban, "~urban <word / phrase>"},
		"google":      {handleGoogle, "~google <word / phrase>"},
		"image":       {handleImage, "~image <word / phrase>"},
		"convert":     {handleConvert, "~convert <time> <IANA timezone>\nhttps://en.wikipedia.org/wiki/List_of_tz_database_time_zones"},
		"perk":        {handlePerk, "~perk <perk name>"},
		"shrine":      {handleShrine, "~shrine"},
		"autoshrine":  {handleAutoshrine, "~autoshrine set #channel / ~autoshrine reset"},
		"help":        {handleHelp, "~help"},
		"wiki":        {handleWiki, "~wiki <word / phrase>"},
		"about":       {attemptAbout, "~about @user"},
		"activity":    {activity, "~activity help"},
		"leaderboard": {leaderboard, "~leaderboard"},
		"greeter":     {greeter, "~greeter help"},
		"modlog":      {setModLogChannel, "~modlog set #channel / ~modlog reset"},
		"addon":       {handleAddon, "~addon <addon name>"},
		"killer":      {handleKiller, "~killer <killer name>"},
		"survivor":    {handleSurvivor, "~survivor <survivor name>"},
		"emoji":       {emoji, "~emoji help"},
		"vcdeaf":      {vcDeaf, "~vcdeaf @user"},
		"vcmute":      {vcMute, "~vcmute @user"},
		"vcmove":      {vcMove, "~vcmove @user #!channel"},
		"vckick":      {vcKick, "~vckick @user"},
	}
}

func runBot(token string) {
	// FOR DEBUG INFO!
	debug = true

	fmt.Println(discordgo.VERSION)

	logInfo("Starting the application")

	// Playing...
	statuses := [...]string{
		"VSCode",
		"with print statements instead of debugging properly",
		"video games instead of doing my classwork",
		"with scissors",
		"Hentai Killer VR",
		"with people who unironically use Ripcord",
		"with unsafe syscalls",
		"PuTTY",
		"Human Simulator 2",
	}

	dbUsername = os.Getenv("DB_USERNAME")
	dbPassword = os.Getenv("DB_PASSWORD")
	dbName = os.Getenv("DB")
	dbHost = os.Getenv("DB_HOST")
	activityTable = os.Getenv("ACTIVITY_TABLE")
	leaderboardTable = os.Getenv("LEADERBOARD_TABLE")
	joinLeaveTable = os.Getenv("JOIN_LEAVE_TABLE")
	autokickTable = os.Getenv("AUTOKICK_TABLE")
	modLogTable = os.Getenv("MODLOG_TABLE")
	autoshrineTable = os.Getenv("AUTOSHRINE_TABLE")

	// open connection to database
	retry := 90
	var db *sql.DB
	var err error
	dbConnectStr := fmt.Sprintf("%s:%s@tcp(%s:3306)/%s", dbUsername, dbPassword, dbHost, dbName)
	logInfo(dbConnectStr)
	for db == nil && retry > 0 {
		logInfo("Calling sql.open")
		db, err = sql.Open("mysql", dbConnectStr)
		if err != nil {
			logWarning("Unable to open DB connection! " + err.Error())
		}
		time.Sleep(1 * time.Second)
		retry--
	}
	if db == nil {
		logError("Could not open DB connection after many retries. Shutting down.")
		return
	} else {
		logInfo("Connected to database.")
	}
	defer db.Close()
	connection_pool = db

	// create tables if they don't exist
	attemptQuery("CREATE TABLE IF NOT EXISTS "+activityTable+" (entry int(11) NOT NULL AUTO_INCREMENT PRIMARY KEY, guild_id char(20), member_id char(20), member_name char(40), last_active char(70), description char(80), whitelist boolean);",
		"Created activity table",
		"Unable to create activity table")
	attemptQuery("CREATE TABLE IF NOT EXISTS "+leaderboardTable+" (entry int(11) NOT NULL AUTO_INCREMENT PRIMARY KEY, guild_id char(20), member_id char(20), member_name char(40), points int(11), last_awarded char(70));",
		"Created leaderboard table",
		"Failed to create leaderboard table")
	attemptQuery("CREATE TABLE IF NOT EXISTS "+joinLeaveTable+" (entry int(11) NOT NULL AUTO_INCREMENT PRIMARY KEY, guild_id char(20), channel_id char(20), message_type char(5), image_link varchar(1000), message varchar(2000));",
		"Created join/leave table",
		"Failed to create join/leave table")
	attemptQuery("CREATE TABLE IF NOT EXISTS "+autokickTable+" (guild_id char(20) PRIMARY KEY, days_until_kick int(11));",
		"Created autokick table",
		"Failed to create autokick table")
	attemptQuery("CREATE TABLE IF NOT EXISTS "+modLogTable+" (guild_id char(20) PRIMARY KEY, channel_id char(20));",
		"Created mod log table",
		"Failed to create mod log table")
	attemptQuery("CREATE TABLE IF NOT EXISTS "+autoshrineTable+" (guild_id char(20) PRIMARY KEY, channel_id char(20));",
		"Created auto shrine table",
		"Failed to create auto shrine table")

	/** Open Connection to Discord **/
	if os.Getenv("PROD_MODE") == "true" {
		logWarning("Production mode is active")
		prodMode = true
	}
	start = time.Now()

	// initialize bot
	dg, err := discordgo.New("Bot " + token)
	if err != nil {
		logError("Error creating discord session... " + err.Error())
		return
	}

	// add listeners
	dg.AddHandler(messageCreate)
	dg.AddHandler(messageReactionAdd)
	dg.AddHandler(guildMemberAdd)
	dg.AddHandler(guildMemberRemove)
	dg.AddHandler(guildCreate)
	dg.AddHandler(guildDelete)
	dg.AddHandler(guildBanAdd)
	dg.AddHandler(guildBanRemove)
	dg.AddHandler(guildEmojisUpdate)
	dg.AddHandler(voiceStateUpdate)

	dg.Identify.Intents = discordgo.IntentsAllWithoutPrivileged | discordgo.IntentsGuildMembers | discordgo.IntentsGuilds

	// open connection to discord
	err = dg.Open()
	if err != nil {
		logError("Error opening connection! " + err.Error())
		return
	}

	initCommandInfo()

	// start rotating statuses
	go rotateStatuses(dg, statuses[:])

	// start auto-kick listener
	go runAutoKicker(dg)

	// start waiting for the new shrine
	go runNewShrineDetection(dg)

	// Wait here until CTRL-C or other term signal is received.
	fmt.Println("Bot is now running.  Press CTRL-C to exit.")
	sc := make(chan os.Signal, 1)
	signal.Notify(sc, syscall.SIGINT, syscall.SIGTERM, os.Interrupt)
	<-sc

	// Cleanly close down the Discord session
	dg.Close()
}

func rotateStatuses(dg *discordgo.Session, statuses []string) {
	for {
		rand.Seed(time.Now().Unix())
		n := rand.Intn(len(statuses))
		err := dg.UpdateGameStatus(0, statuses[n])
		if err != nil {
			logError("Failed to select a new game status! " + err.Error())
		} else {
			logSuccess("Updated game status to: " + statuses[n])
		}

		time.Sleep(2 * time.Hour)
	}
}

func runAutoKicker(dg *discordgo.Session) {
	for {
		logWarning("Performing auto-kick")
		// 1. get days_until_kick for each guild
		query, err := connection_pool.Query(fmt.Sprintf("SELECT * FROM %s;", autokickTable))
		if err != nil {
			logError("SELECT query error: " + err.Error())
			return
		}

		for query.Next() {
			var autokickData AutoKickData
			err = query.Scan(&autokickData.GuildID, &autokickData.DaysUntilKick)
			if err != nil {
				logError("Unable to parse database information! Aborting. " + err.Error())
				return
			}

			// 2. get all users from member activity table that are not whitelisted and are in the given guild
			memberQuery, err := connection_pool.Query(fmt.Sprintf("SELECT * FROM %s WHERE (guild_id = ? AND whitelist = false);", activityTable), autokickData.GuildID)
			if err != nil {
				logError("SELECT query error: " + err.Error())
				return
			}

			for memberQuery.Next() {
				var memberActivity MemberActivity
				err = memberQuery.Scan(&memberActivity.ID, &memberActivity.GuildID, &memberActivity.MemberID, &memberActivity.MemberName, &memberActivity.LastActive, &memberActivity.Description, &memberActivity.Whitelisted)
				if err != nil {
					logError("Unable to parse database information! Aborting. " + err.Error())
					continue
				}
				dateFormat := "2006-01-02 15:04:05.999999999 -0700 MST"
				// calculate difference between time.Now() and the provided timestamp
				lastActive, err := time.Parse(dateFormat, strings.Split(memberActivity.LastActive, " m=")[0])
				if err != nil {
					logError("Unable to parse database timestamps! Aborting. " + err.Error())
					return
				}

				lastActive = lastActive.AddDate(0, 0, autokickData.DaysUntilKick)
				if lastActive.Before(time.Now()) {
					// kick user
					err = dg.GuildMemberDeleteWithReason(autokickData.GuildID, memberActivity.MemberID, fmt.Sprintf("Bot detected %d or more days of inactivity.", autokickData.DaysUntilKick))
					if err != nil {
						logError("Unable to kick user! " + err.Error())
					}
					guild, err := dg.Guild(autokickData.GuildID)
					if err != nil {
						logError("Unable to load guild! " + err.Error())
					}
					guildName := "error: could not retrieve"
					if guild != nil {
						guildName = guild.Name
					}
					dmUser(dg, memberActivity.MemberID, fmt.Sprintf("You have been automatically kicked from **%s** due to %d or more days of inactivity.", guildName, autokickData.DaysUntilKick))
				}
			}
		}
		query.Close()

		time.Sleep(6 * time.Hour)
	}
}

/**
Gets the end date of the existing shrine, waits until a few minutes after that, then posts the new shrine in all channels where ~autoshrine was configured.
*/
func runNewShrineDetection(dg *discordgo.Session) {
	shrine := scrapeShrine()
	end_time_unix := shrine.End + 600 // add 10 minute buffer period
	for {
		if end_time_unix < time.Now().Unix() {
			handleShrineUpdate(dg)
			shrine = scrapeShrine()
			end_time_unix = shrine.End
			end_time_unix += 600 // add 10 minute buffer period
		}
		time.Sleep(15 * time.Minute)
	}
}

/**
Creates an embed displaying all the potential commands and their functions.
*/
func handleHelp(s *discordgo.Session, m *discordgo.MessageCreate, command []string) {
	// return all current commands and what they do
	var embed discordgo.MessageEmbed
	embed.Type = "rich"

	embed.Title = "❓ How to Use AiO Bot ❓"

	// add a cute thumbnail
	var thumbnail discordgo.MessageEmbedThumbnail
	thumbnail.URL = "https://img.pngio.com/robot-icon-of-flat-style-available-in-svg-png-eps-ai-icon-robot-icon-png-256_256.png"
	embed.Thumbnail = &thumbnail

	// add all commands to the embed as a set of fields that are not inline
	embed.Description = "The [Command List](https://cazwacki.github.io/bot-commands.html) is now hosted on Github.IO"

	// self-credit + github profile picture
	var footer discordgo.MessageEmbedFooter
	footer.Text = "Created by Charles Zawacki; Written in Go"
	footer.IconURL = "https://avatars0.githubusercontent.com/u/44577941?s=460&u=4eb7b9ff5410be189eea9863c33916c805dbd2b2&v=4"
	embed.Footer = &footer

	// send response
	_, err := s.ChannelMessageSendEmbed(m.ChannelID, &embed)
	if err != nil {
		logError("Unable to send message! " + err.Error())
	}
}

/**
Handler function when the discord session detects a message is created in
a channel that the bot has access to.
*/
func messageCreate(s *discordgo.Session, m *discordgo.MessageCreate) {
	logInfo("Message Create Event")
	go checkForMessageLink(s, m)
	go logActivity(m.GuildID, m.Author, time.Now().String(), "Wrote a message in <#"+m.ChannelID+">", false)
	awardPoints(m.GuildID, m.Author, time.Now().String(), m.Content)
	respondToCommands(s, m)
}

/**
Used to handle scrolling through images given from ~image,
but can and may be used to handle other reactions in the future
*/
func messageReactionAdd(s *discordgo.Session, m *discordgo.MessageReactionAdd) {
	go navigateImages(s, m)
	user, err := s.User(m.UserID)
	if err != nil {
		logError("Could not get the user from the session state! " + err.Error())
		return
	}
	go logActivity(m.GuildID, user, time.Now().String(), "Reacted with :"+m.Emoji.Name+": to a message in <#"+m.ChannelID+">", false)
}

func guildMemberAdd(s *discordgo.Session, m *discordgo.GuildMemberAdd) {
	go logActivity(m.GuildID, m.User, time.Now().String(), "Joined the server", true)
	go joinLeaveMessage(s, m.GuildID, m.User, "join")
}

func guildMemberRemove(s *discordgo.Session, m *discordgo.GuildMemberRemove) {
	logInfo("Guild Member Remove Event")
	go removeUser(m.GuildID, m.User.ID)
	latestLog, err := s.GuildAuditLog(m.GuildID, "", "", -1, 1)
	if err != nil {
		logError("Could not get the guild audit log from the session state! " + err.Error())
		return
	}
	// log mod activity if applicable
	if latestLog.AuditLogEntries[0].TargetID == m.User.ID && *latestLog.AuditLogEntries[0].ActionType == discordgo.AuditLogActionMemberKick {
		go logModActivity(s, m.GuildID, latestLog.AuditLogEntries[0])
	}
	joinLeaveMessage(s, m.GuildID, m.User, "leave")
}

func guildCreate(s *discordgo.Session, m *discordgo.GuildCreate) {
	logNewGuild(s, m.ID)
}

func guildDelete(s *discordgo.Session, m *discordgo.GuildDelete) {
	removeGuild(m.ID)
}

func guildEmojisUpdate(s *discordgo.Session, m *discordgo.GuildEmojisUpdate) {
	// check audit log for who did it
	latestLog, err := s.GuildAuditLog(m.GuildID, "", "", -1, 1)
	if err != nil {
		logError("Could not get the guild audit log from the session state! " + err.Error())
		return
	}
	fmt.Printf("%+v\n", latestLog)
	fmt.Printf("%+v\n", latestLog.AuditLogEntries[0])
	fmt.Printf("%+v\n", latestLog.AuditLogEntries[0].Changes[0])
}

func guildBanAdd(s *discordgo.Session, m *discordgo.GuildBanAdd) {
	logInfo("Guild Ban Added")
	time.Sleep(time.Millisecond * 2000)
	latestLog, err := s.GuildAuditLog(m.GuildID, "", "", (int)(discordgo.AuditLogActionMemberBanAdd), 1)
	if err != nil {
		logError("Could not get the guild audit log from the session state! " + err.Error())
		return
	}
	if latestLog.AuditLogEntries[0].TargetID == m.User.ID {
		go logModActivity(s, m.GuildID, latestLog.AuditLogEntries[0])
	}
}

func guildBanRemove(s *discordgo.Session, m *discordgo.GuildBanRemove) {
	logInfo("Guild Ban Removed")
	latestLog, err := s.GuildAuditLog(m.GuildID, "", "", (int)(discordgo.AuditLogActionMemberBanRemove), 1)
	if err != nil {
		logError("Could not get the guild audit log from the session state! " + err.Error())
		return
	}
	if latestLog.AuditLogEntries[0].TargetID == m.User.ID {
		go logModActivity(s, m.GuildID, latestLog.AuditLogEntries[0])
	}
}

func voiceStateUpdate(s *discordgo.Session, v *discordgo.VoiceStateUpdate) {
	user, err := s.User(v.UserID)
	if err != nil {
		logError("Could not get the user from the session state! " + err.Error())
		return
	}
	if v.ChannelID == "" {
		if v.BeforeUpdate != nil {
			logActivity(v.GuildID, user, time.Now().String(), "Left <#"+v.BeforeUpdate.ChannelID+">", false)
		} else {
			logActivity(v.GuildID, user, time.Now().String(), "Left a voice channel", false)
		}
	} else {
		logActivity(v.GuildID, user, time.Now().String(), "Joined <#"+v.ChannelID+">", false)
	}
}

func checkForMessageLink(s *discordgo.Session, m *discordgo.MessageCreate) {
	// Ignore my testing channel
	if prodMode && m.ChannelID == "739852388264968243" {
		return
	}
	// Ignore all messages created by the bot itself as well as DMs
	if m.Author.ID == s.State.User.ID || m.GuildID == "" {
		return
	}
	regex := regexp.MustCompile(`https:\/\/discord.com\/channels\/[0-9]*\/[0-9]*\/[0-9]*`)
	match := regex.FindAllStringSubmatch(m.Content, -1)
	logInfo(fmt.Sprintf("Number of matches is %d", len(match)))

	// verify the message came from within the guild
	for _, link := range match {
		linkData := strings.Split(link[0], "/")
		fmt.Printf("%s\n", linkData[5])
		if linkData[4] == m.GuildID {
			var embed discordgo.MessageEmbed
			embed.Type = "rich"

			linkedMessage, err := s.ChannelMessage(linkData[5], linkData[6])
			if err != nil {
				logError("Unable to pull message from session: " + err.Error())
				return
			}

			// populating author information in the embed
			var embedAuthor discordgo.MessageEmbedAuthor
			if linkedMessage.Author != nil {
				member, err := s.GuildMember(linkedMessage.GuildID, linkedMessage.Author.ID)
				nickname := ""
				if err == nil {
					nickname = member.Nick
				} else {
					logWarning("Unable to retrieve member's nickname. " + err.Error())
				}
				embedAuthor.Name = ""
				if nickname != "" {
					embedAuthor.Name += nickname + " ("
				}
				embedAuthor.Name += linkedMessage.Author.Username + "#" + linkedMessage.Author.Discriminator
				if nickname != "" {
					embedAuthor.Name += ")"
				}
				embedAuthor.IconURL = linkedMessage.Author.AvatarURL("")
			}
			embed.Author = &embedAuthor

			// add user's message information
			embed.Description = linkedMessage.Content
			embed.Timestamp = linkedMessage.Timestamp.Format("2006-01-02T15:04:05-0700")

			linkedMessageChannel, err := s.Channel(linkData[5])
			if err != nil {
				logError("Unable to pull channel from session: " + err.Error())
				return
			}

			var contents []*discordgo.MessageEmbedField

			// output attachments
			if len(linkedMessage.Attachments) > 0 {
				for i, attachment := range linkedMessage.Attachments {
					title := fmt.Sprintf("Attachment %d: %s", i+1, attachment.Filename)
					contents = append(contents, createField(title, attachment.ProxyURL, false))
				}
			}

			embed.Fields = contents

			var footer discordgo.MessageEmbedFooter
			footer.Text = "in #" + linkedMessageChannel.Name
			embed.Footer = &footer

			// send response
			_, err = s.ChannelMessageSendEmbed(m.ChannelID, &embed)
			if err != nil {
				logError("Failed to send message link embed! " + err.Error())
				return
			}
			logSuccess("Sent message link embed")
		}
	}
}

func respondToCommands(s *discordgo.Session, m *discordgo.MessageCreate) {
	// Ignore my testing channel
	if prodMode && m.ChannelID == "739852388264968243" {
		return
	}
	// Ignore all messages created by the bot itself as well as DMs
	if m.Author.ID == s.State.User.ID || m.GuildID == "" {
		return
	}

	parsedCommand := strings.Split(m.Content, " ")
	if !strings.HasPrefix(parsedCommand[0], prefix) {
		return
	}
	invoke_word := strings.TrimPrefix(parsedCommand[0], prefix)

	// get the command information based on the invoke word
	if validCommand, ok := commandList[invoke_word]; ok {
		// special case: needs to pass in starting time
		if parsedCommand[0] == prefix+"uptime" {
			parsedCommand = []string{start.Format("2006-01-02 15:04:05.999999999 -0700 MST")}
		}
		go validCommand.handle(s, m, parsedCommand)
	}
}

func navigateImages(s *discordgo.Session, m *discordgo.MessageReactionAdd) {
	// Ignore all messages created by the bot itself as well as DMs
	if m.UserID == s.State.User.ID {
		return
	}
	for i, set := range globalImageSet {
		if set.Message.ID == m.MessageID {
			if m.Emoji.Name == "⬅️" || m.Emoji.Name == "➡️" || m.Emoji.Name == "⏹️" {
				/*
					1. Remove the reaction the user made
					2. Switch the image if the index allows
					3. Update the index in the set
				*/
				if m.Emoji.Name == "⏹️" {
					logInfo("Removing listener for message ID " + m.MessageID)
					tmpSet := globalImageSet[0]
					globalImageSet[0] = globalImageSet[i]
					globalImageSet[i] = tmpSet
					globalImageSet = globalImageSet[1:]
					err := s.MessageReactionsRemoveAll(m.ChannelID, m.MessageID)
					if err != nil {
						logError("Failed to remove all reactions from message! " + err.Error())
						return
					}
					logSuccess("Removed from image sets and removed reactions on the message")
				} else {
					// craft response and send
					var embed discordgo.MessageEmbed
					embed.Type = "rich"
					embed.Title = "Image Results for \"" + set.Query + "\""

					fmt.Printf("Index before: %d\n", set.Index)
					if m.Emoji.Name == "⬅️" {
						if set.Index != 0 {
							logInfo("Previous image for message ID " + m.MessageID)
							set.Index--
						}
					} else if m.Emoji.Name == "➡️" {
						if set.Index != len(set.Images)-1 {
							logInfo("Next image for message ID " + m.MessageID)
							set.Index++
						}
					}
					fmt.Printf("Index after: %d\n", set.Index)

					var image discordgo.MessageEmbedImage
					image.URL = set.Images[set.Index]
					embed.Image = &image
					var footer discordgo.MessageEmbedFooter
					footer.Text = fmt.Sprintf("Image %d of %d", set.Index+1, len(set.Images))
					footer.IconURL = "https://cdn4.iconfinder.com/data/icons/new-google-logo-2015/400/new-google-favicon-512.png"
					embed.Footer = &footer
					_, err := s.ChannelMessageEditEmbed(m.ChannelID, m.MessageID, &embed)
					if err != nil {
						logError("Failed to send edit image embed! " + err.Error())
						return
					}
					err = s.MessageReactionRemove(m.ChannelID, m.MessageID, m.Emoji.Name, m.UserID)
					if err != nil {
						logError("Failed to remove user's reaction! " + err.Error())
						return
					}
					logSuccess("Updated image embed")
				}
			}
		}
	}
	for _, set := range globalInactiveSet {
		if set.Message.ID == m.MessageID {
			if m.Emoji.Name == "◀️" || m.Emoji.Name == "▶️" {
				// craft response and send
				var embed discordgo.MessageEmbed
				embed.Type = "rich"
				if set.DaysInactive > 0 {
					embed.Title = "Users Inactive for " + strconv.Itoa(set.DaysInactive) + "+ Days"
				} else {
					embed.Title = "User Activity"
				}

				pageCount := len(set.Inactives) / 8
				if len(set.Inactives)%8 != 0 {
					pageCount++
				}

				if m.Emoji.Name == "◀️" {
					if set.Index != 0 {
						logInfo("Previous page for message ID " + m.MessageID)
						set.Index--
					}
				} else if m.Emoji.Name == "▶️" {
					if set.Index != pageCount-1 {
						logInfo("Next page for message ID " + m.MessageID)
						set.Index++
					}
				}

				if set.Index >= 0 && set.Index < pageCount {
					var contents []*discordgo.MessageEmbedField
					for i := set.Index * 8; i < set.Index*8+8 && i < len(set.Inactives); i++ {
						// calculate difference between time.Now() and the provided timestamp
						dateFormat := "2006-01-02 15:04:05.999999999 -0700 MST"
						lastActive, err := time.Parse(dateFormat, strings.Split(set.Inactives[i].LastActive, " m=")[0])
						if err != nil {
							logError("Unable to parse database timestamps! Aborting. " + err.Error())
							return
						}
						fieldValue := "- " + lastActive.Format("01/02/2006 15:04:05") + "\n- " + set.Inactives[i].Description
						// add whitelist state
						if set.Inactives[i].Whitelisted == 1 {
							fieldValue += "\n- Protected from auto-kick"
						}

						contents = append(contents, createField(set.Inactives[i].MemberName, fieldValue, false))
					}
					embed.Fields = contents

					var footer discordgo.MessageEmbedFooter
					footer.Text = fmt.Sprintf("Page %d of %d", set.Index+1, pageCount)
					embed.Footer = &footer

					s.ChannelMessageEditEmbed(m.ChannelID, m.MessageID, &embed)
					s.MessageReactionRemove(m.ChannelID, m.MessageID, m.Emoji.Name, m.UserID)
				}
			}
		}
	}
}
