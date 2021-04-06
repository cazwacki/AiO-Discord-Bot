package main

import (
	"database/sql"
	"fmt"
	"net/url"
	"os"
	"os/signal"
	"regexp"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/ChimeraCoder/anaconda"
	"github.com/bwmarrin/discordgo"
)

var start time.Time
var prodMode bool
var globalImageSet []*ImageSet
var globalInactiveSet []*InactiveSet
var prefix string
var commandList map[string]command

type handler func(*discordgo.Session, *discordgo.MessageCreate, []string)

type command struct {
	handle handler
}

func appendToGlobalImageSet(s *discordgo.Session, newset ImageSet) {
	globalImageSet = append(globalImageSet, &newset)
	fmt.Println("Global Image Set:")
	fmt.Println(globalImageSet)

	time.Sleep(30 * time.Minute)

	for i, set := range globalImageSet {
		if &newset == set {
			tmpSet := globalImageSet[0]
			globalImageSet[0] = globalImageSet[i]
			globalImageSet[i] = tmpSet
			globalImageSet = globalImageSet[1:]
			s.MessageReactionsRemoveAll(newset.Message.ChannelID, newset.Message.ID)
		}
	}
}

func appendToGlobalInactiveSet(s *discordgo.Session, newset InactiveSet) {
	globalInactiveSet = append(globalInactiveSet, &newset)
	fmt.Println("Global Inactive Set:")
	fmt.Println(globalInactiveSet)

	time.Sleep(30 * time.Minute)

	for i, set := range globalInactiveSet {
		if &newset == set {
			tmpSet := globalInactiveSet[0]
			globalInactiveSet[0] = globalInactiveSet[i]
			globalInactiveSet[i] = tmpSet
			globalInactiveSet = globalInactiveSet[1:]
			s.MessageReactionsRemoveAll(newset.Message.ChannelID, newset.Message.ID)
		}
	}
}

/**
Initialize command information and prefix
*/
func initCommandInfo() {
	prefix = "~"
	commandList = map[string]command{
		"shutdown":    {handleShutdown},
		"invite":      {handleInvite},
		"profile":     {attemptProfile},
		"nick":        {handleNickname},
		"kick":        {handleKick},
		"ban":         {handleBan},
		"mv":          {handleMove},
		"cp":          {handleCopy},
		"purge":       {handlePurge},
		"define":      {handleDefine},
		"google":      {handleGoogle},
		"image":       {handleImage},
		"perk":        {handlePerk},
		"shrine":      {handleShrine},
		"autoshrine":  {handleAutoshrine},
		"help":        {handleHelp},
		"wiki":        {handleWiki},
		"about":       {attemptAbout},
		"activity":    {activity},
		"leaderboard": {leaderboard},
		"greeter":     {greeter},
	}
}

func runBot(token string) {
	dbUsername = os.Getenv("DB_USERNAME")
	dbPassword = os.Getenv("DB_PASSWORD")
	db = os.Getenv("DB")
	activityTable = os.Getenv("ACTIVITY_TABLE")
	leaderboardTable = os.Getenv("LEADERBOARD_TABLE")
	joinLeaveTable = os.Getenv("JOIN_LEAVE_TABLE")

	// open connection to database
	db, err := sql.Open("mysql", fmt.Sprintf("%s:%s@tcp(localhost:3306)/%s", dbUsername, dbPassword, db))
	if err != nil {
		fmt.Println("Unable to open DB connection! " + err.Error())
		return
	}
	defer db.Close()
	connection_pool = db

	// create tables if they don't exist
	createActivityTableSQL := fmt.Sprintf("CREATE TABLE IF NOT EXISTS %s (entry int(11) NOT NULL AUTO_INCREMENT PRIMARY KEY, guild_id char(20), member_id char(20), member_name char(40), last_active char(70), description char(80));", activityTable)
	createLeaderboardTableSQL := fmt.Sprintf("CREATE TABLE IF NOT EXISTS %s (entry int(11) NOT NULL AUTO_INCREMENT PRIMARY KEY,	guild_id char(20), member_id char(20), member_name char(40), points int(11), last_awarded char(70));", leaderboardTable)
	createJoinLeaveTableSQL := fmt.Sprintf("CREATE TABLE IF NOT EXISTS %s (entry int(11) NOT NULL AUTO_INCREMENT PRIMARY KEY, guild_id char(20), channel_id char(20), message_type char(5), image_link varchar(1000), message varchar(2000));", joinLeaveTable)
	queryWithoutResults(createActivityTableSQL, "Unable to create activity table!")
	queryWithoutResults(createLeaderboardTableSQL, "Unable to create leaderboard table!")
	queryWithoutResults(createJoinLeaveTableSQL, "Unable to create join / leave table!")

	/** Open Connection to Discord **/
	if os.Getenv("PROD_MODE") == "true" {
		prodMode = true
	}
	start = time.Now()

	// initialize bot
	dg, err := discordgo.New("Bot " + token)
	if err != nil {
		fmt.Println("Error creating discord session")
		return
	}

	// add listeners
	dg.AddHandler(messageCreate)
	dg.AddHandler(messageReactionAdd)
	dg.AddHandler(guildMemberAdd)
	dg.AddHandler(guildMemberRemove)
	dg.AddHandler(guildCreate)
	dg.AddHandler(guildDelete)

	*dg.Identify.Intents = discordgo.IntentsAllWithoutPrivileged | discordgo.IntentsGuildMembers | discordgo.IntentsGuilds

	// open connection to discord
	err = dg.Open()
	if err != nil {
		fmt.Println("Error opening connection,", err)
		return
	}

	initCommandInfo()

	/** Open Connection to Twitter **/
	anaconda.SetConsumerKey(os.Getenv("TWITTER_API_KEY"))
	anaconda.SetConsumerSecret(os.Getenv("TWITTER_API_SECRET"))
	api := anaconda.NewTwitterApi(os.Getenv("TWITTER_TOKEN"), os.Getenv("TWITTER_TOKEN_SECRET"))
	go runTwitterLoop(api, dg)

	// Wait here until CTRL-C or other term signal is received.
	fmt.Println("Bot is now running.  Press CTRL-C to exit.")
	sc := make(chan os.Signal, 1)
	signal.Notify(sc, syscall.SIGINT, syscall.SIGTERM, os.Interrupt)
	<-sc

	// Cleanly close down the Discord session and Twitter connection.
	dg.Close()
	api.Close()
}

/**
Opens a stream looking for new tweets from @DeadbyBHVR, who posts the weekly
shrine on Twitter.
*/
func runTwitterLoop(api *anaconda.TwitterApi, dg *discordgo.Session) {
	fmt.Println("Starting...")
	v := url.Values{}
	v.Set("follow", "4850837842") // @DeadbyBHVR is 4850837842
	v.Set("track", "shrine")
	s := api.PublicStreamFilter(v)
	for t := range s.C {
		switch v := t.(type) {
		case anaconda.Tweet:
			handleTweet(dg, v)
		}
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
	embed.Description = "[Command List](https://cazwacki.github.io/bot-commands.html)"

	// self-credit + github profile picture
	var footer discordgo.MessageEmbedFooter
	footer.Text = "Created by Charles Zawacki; Written in Go"
	footer.IconURL = "https://avatars0.githubusercontent.com/u/44577941?s=460&u=4eb7b9ff5410be189eea9863c33916c805dbd2b2&v=4"
	embed.Footer = &footer

	// send response
	s.ChannelMessageSendEmbed(m.ChannelID, &embed)

}

/**
Handler function when the discord session detects a message is created in
a channel that the bot has access to.
*/
func messageCreate(s *discordgo.Session, m *discordgo.MessageCreate) {
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
		fmt.Println("Error grabbing the user from m.UserID; " + err.Error())
		return
	}
	go logActivity(m.GuildID, user, time.Now().String(), "Reacted with :"+m.Emoji.Name+": to a message in <#"+m.ChannelID+">", false)
}

func guildMemberAdd(s *discordgo.Session, m *discordgo.GuildMemberAdd) {
	fmt.Println("User was added to guild")
	go logActivity(m.GuildID, m.User, time.Now().String(), "Joined the server", true)
	go joinLeaveMessage(s, m.GuildID, m.User, "join")
}

func guildMemberRemove(s *discordgo.Session, m *discordgo.GuildMemberRemove) {
	fmt.Println("User was removed from guild")
	go removeUser(m.GuildID, m.User.ID)
	go joinLeaveMessage(s, m.GuildID, m.User, "leave")
}

func guildCreate(s *discordgo.Session, m *discordgo.GuildCreate) {
	go logNewGuild(s, m.ID)
}

func guildDelete(s *discordgo.Session, m *discordgo.GuildDelete) {
	removeGuild(m.ID)
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
	regex := regexp.MustCompile(`https:\/\/discord.com\/channels\/[0-9]{18}\/[0-9]{18}\/[0-9]{18}`)
	match := regex.FindStringSubmatch(m.Content)
	if match != nil {
		// verify the message came from within the guild
		linkData := strings.Split(match[0], "/")
		if linkData[4] == m.GuildID {
			var embed discordgo.MessageEmbed
			embed.Type = "rich"

			linkedMessage, err := s.ChannelMessage(linkData[5], linkData[6])
			if err != nil {
				fmt.Println("ERROR linking message: " + err.Error())
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
					fmt.Println(err)
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
			embed.Timestamp = string(linkedMessage.Timestamp)

			linkedMessageChannel, err := s.Channel(linkData[5])
			if err != nil {
				fmt.Println("ERROR linking message: " + err.Error())
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
			s.ChannelMessageSendEmbed(m.ChannelID, &embed)
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
					// remove reactions and remove from list
					tmpSet := globalImageSet[0]
					globalImageSet[0] = globalImageSet[i]
					globalImageSet[i] = tmpSet
					globalImageSet = globalImageSet[1:]
					s.MessageReactionsRemoveAll(m.ChannelID, m.MessageID)
				} else {
					// craft response and send
					var embed discordgo.MessageEmbed
					embed.Type = "rich"
					embed.Title = "Image Results for \"" + set.Query + "\""

					fmt.Printf("Index before: %d\n", set.Index)
					if m.Emoji.Name == "⬅️" {
						if set.Index != 0 {
							set.Index--
						}
					} else if m.Emoji.Name == "➡️" {
						if set.Index != len(set.Images)-1 {
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
					s.ChannelMessageEditEmbed(m.ChannelID, m.MessageID, &embed)
					s.MessageReactionRemove(m.ChannelID, m.MessageID, m.Emoji.Name, m.UserID)
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
						set.Index--
					}
				} else if m.Emoji.Name == "▶️" {
					if set.Index != pageCount-1 {
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
							fmt.Println("Unable to parse database timestamps! Aborting. " + err.Error())
							return
						}
						contents = append(contents, createField(set.Inactives[i].MemberName, "- "+lastActive.Format("01/02/2006 15:04:05")+"\n- "+set.Inactives[i].Description, false))
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
