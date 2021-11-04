[![Go Report Card](https://goreportcard.com/badge/github.com/cazwacki/PersonalDiscordBot)](https://goreportcard.com/report/github.com/cazwacki/PersonalDiscordBot) [![Build Status](https://travis-ci.org/cazwacki/PersonalDiscordBot.svg?branch=master)](https://travis-ci.org/cazwacki/PersonalDiscordBot)

# PersonalDiscordBot
Bot for my Discord server hooked up to a CI/CD system and developed in Go. It utilizes web scraping to get some of its information.

UPDATE: No longer going to be using Travis-CI as of 12/11/2020. Not going to pay $70/mo. for a pet project.

## Setup
1. Install [Docker](https://docs.docker.com/get-docker/) and [Docker Compose](https://docs.docker.com/compose/install/) if you don't have them.
2. Run `docker pull czawacki/aio-bot` to download the build.
3. [Create a bot on the Discord Developer Portal](https://discord.com/developers) and save its token.
4. (~autoshrine functionality) [Get Twitter API access.](https://developer.twitter.com/en/apply-for-access) You need an API Key, Secret, Twitter Token, and Twitter Token Secret.
5. (~image functionality) [Get Google CustomSearch API Access.](https://developers.google.com/custom-search/v1/overview) You need a Google API Key. (Only the first 100 requests each day are free, so I would only use this bot on a server with a few people.)
6. (~define functionality) [Get a Lingua API Key.](https://www.linguarobot.io/) The first 2500 requests a day are free.
7. (~urban functionality) [Get an unofficial Urban Dictionary API Key.](https://rapidapi.com/community/api/urban-dictionary)
8. Put the keys, tokens, and secrets you have acquired into api-keys.env.
9. Configure your MariaDB volume location in docker-compose.yml.
10. `cd` into the project and call `docker-compose up -d` (-d is optional; it makes the containers run in the background). The bot should start running after a couple minutes the first time; afterwards, it should only be a few seconds each time the bot is started.

## Commands

(The bot detects message links. If the source message is in the guild, it will output it in the chat after the user's message.)

Commands are available at https://charles.zawackis.com/bot-commands.html.

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
