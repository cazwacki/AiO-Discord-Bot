[![Go Report Card](https://goreportcard.com/badge/github.com/cazwacki/PersonalDiscordBot)](https://goreportcard.com/report/github.com/cazwacki/PersonalDiscordBot) [![Build Status](https://travis-ci.org/cazwacki/PersonalDiscordBot.svg?branch=master)](https://travis-ci.org/cazwacki/PersonalDiscordBot)

# PersonalDiscordBot
Bot for my Discord server hooked up to a CI/CD system and developed in Go. It utilizes web scraping to get some of its information.

UPDATE: No longer going to be using Travis-CI as of 12/11/2020. Not going to pay $70/mo. for a pet project.

## Setup
1. If your environment does not have Go, [install it from here.](https://golang.org/dl/)
2. Clone/download this code.
3. [Create a bot on the Discord Developer Portal](https://discord.com/developers) and save its token.
4. (~autoshrine functionality) [Get Twitter API access.](https://developer.twitter.com/en/apply-for-access) You need an API Key, Secret, Twitter Token, and Twitter Token Secret.
5. (~image functionality) [Get Google CustomSearch API Access.](https://developers.google.com/custom-search/v1/overview) You need a Google API Key. (Only the first 100 requests each day are free, so I would only use this bot on a server with a few people.)
6. (~define functionality) [Get a Lingua API Key.](https://www.linguarobot.io/) The first 2500 requests a day are free.
6. Set the following information as environment variables on the system you are deploying the bot:
   - BOT_TOKEN
   - TWITTER_API_KEY
   - TWITTER_API_SECRET
   - TWITTER_TOKEN
   - TWITTER_TOKEN_SECRET
   - GOOGLE_API_KEY
   - LINGUA_API_KEY
7. Call `go run .` to invoke the bot.

## Commands

### Standard / Management
- [x] ~nick @user (new username): If you have the permissions, nickname the specified user on the server.
- [x] ~kick @user (reason: optional): Kick the specified user from the server.
- [x] ~ban @user (reason: optional): Ban the specified user from the server.
- [x] ~uptime: Reports the bot's current uptime.
- [x] ~shutdown: Shuts down the bot. Note that if the bot is deployed on a webservice like Heroku, it will probably immediately restart by design.
- [x] ~purge (number) (@user: optional): Removes the (number) most recent commands.
- [x] ~mv (number) (#channel): Moves the last (number) messages from the channel it is invoked in and moves them to (#channel).
- [x] ~cp (number) (#channel): Copies the last (number) messages from the channel it is invoked in and moves them to (#channel).
- [x] ~activity list (number): Returns a report of users who have been inactive for (number) days or more.
- [x] ~activity user @user: Returns the user's last sign of activity.
- [x] ~activity rescan: (Should be useless most of the time) Checks for any users in a server that are not in the database, and adds them to it.
- [x] ~about @user: Get user details related to the Guild the message was called in. 
  
### Dead By Daylight Commands
- [x] ~perk (perk name): Scrapes https://deadbydaylight.gamepedia.com/ for the perk and outputs its description.
- [x] ~shrine: Scrapes the current Shrine of Secrets from https://deadbydaylight.gamepedia.com/.
- [x] ~autoshrine (#channel): Changes the channel where Tweets about the newest shrine from @DeadbyBHVR are posted.
  
### Lookup Commands
- [x] ~define (word / phrase): Returns a definition of the word / phrase if it is available.
- [x] ~wiki (word / phrase): Returns the extract of the topic from Wikipedia if it is available.
- [x] ~google (word / phrase): Returns the first five results from Google Search Engine.
- [x] ~image (word / phrase): Returns the first image from Google Search Engine
- [x] ~help: Returns how to use each of the commands the bot has available.

## Things I Have Learned

### Setting up CI/CD
1. I create an issue on Github describing what I want to implement in the bot.
2. I make commits to a Github branch dedicated to adding the specific feature I want to add.
3. Each commit triggers Travis CI to run tests on my commit and evaluate whether the build passes tests.
3. When the feature branch is ready and tests are established, I create a merge request for the branch.
4. I link the merge request to the issue.
5. After Travis CI says that the merge request passes tests and the merge is considered safe by Github, I merge the branch. This also closes the created issue.
6. The master branch is tested by Travis CI. If it passes, the code is automatically deployed to Heroku under a worker dyno.

### Web Scraping
1. I pull the page from whatever site I need to scrape information from.
2. I use goquery (similar to jQuery) to parse and search for information.
3. I use it to populate the messages sent to the channel.

### Consuming Rest API in the intended way with Golang
1. I create structs that will contain the JSON data returned from API queries.
2. I use json.Unmarshal to parse the JSON response into the struct(s).

### New Language: Go
I wanted to learn a new language since we are limited to C and Java in classes. I've done Javascript before, so I decided to write the bot in Go.
