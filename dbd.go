package main

import "github.com/bwmarrin/discordgo"

/**
Fetches perk information from https://deadbydaylight.gamepedia.com/Dead_by_Daylight_Wiki
and displays the gif icon as well as the perk's source (if there is one) and what it does.
**/
func Handle_perk(s *discordgo.Session, m *discordgo.MessageCreate, command []string) {

}

/**
Checks https://deadbydaylight.gamepedia.com/Dead_by_Daylight_Wiki for the most recent shrine
post and outputs its information.
**/
func Handle_shrine(s *discordgo.Session, m *discordgo.MessageCreate) {

}

/**
Switches the channel that the tweet monitoring system will output to.
**/
func Handle_autoshrine(s *discordgo.Session, m *discordgo.MessageCreate) {

}
