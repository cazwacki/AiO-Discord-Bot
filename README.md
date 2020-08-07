# PersonalDiscordBot
Bot for my discord server hooked up to a CI/CD system and developed in Go.

## Commands

### Standard / Management
- [x] ~nick @user <new username>: If you have the permissions, nickname the specified user on the server.
- [x] ~kick @user (reason: optional): Kick the specified user from the server.
- [x] ~ban @user (reason: optional): Ban the specified user from the server.
- [x] ~uptime: Reports the bot's current uptime.
- [x] ~shutdown: Shuts down the bot. Note that if the bot is deployed on a webservice like Heroku, it will probably immediately restart by design.
  
### Dead By Daylight Commands
- [x] ~perk <perk name>: Scrapes https://deadbydaylight.gamepedia.com/ for the perk and outputs its description.
- [x] ~shrine: Scrapes the current Shrine of Secrets from https://deadbydaylight.gamepedia.com/.
- [x] ~autoshrine <#channel>: Changes the channel where Tweets about the newest shrine from @DeadbyBHVR are posted.
  
### Lookup Commands
- [ ] ~define <word / phrase>: Returns a definition of the word / phrase if it is available.
- [ ] ~google <word / phrase>: Returns the first five results from Google Search Engine.
- [ ] ~image <word / phrase>: Returns the first image from Google Search Engine
- [ ] ~help: Returns how to use each of the commands the bot has available.

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

### New Language: Go
I wanted to learn a new language since we are limited to C and Java in classes. I've done Javascript before, so I decided to write the bot in Go.
