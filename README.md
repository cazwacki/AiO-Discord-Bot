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
