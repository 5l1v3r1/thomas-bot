package main

import (
	"fmt"
	"log"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/patrickmn/go-cache"
)

type checkFn func(s *discordgo.Session, m *discordgo.MessageCreate) bool

var checks = []checkFn{
	removeLink,
}

const itfGuestRole = "687568536356257890"

var userCache *cache.Cache
var notifyCache *cache.Cache
var reactionNotifyCache *cache.Cache
var reactionCache *cache.Cache
var checkCache *cache.Cache

func init() {
	userCache = cache.New(5*time.Minute, 10*time.Minute)
	notifyCache = cache.New(time.Minute, 5*time.Minute)
	reactionCache = cache.New(2*time.Minute, 5*time.Minute)
	reactionNotifyCache = cache.New(time.Minute, 5*time.Minute)
	checkCache = cache.New(time.Minute, 5*time.Minute)
}

func checkMessage(s *discordgo.Session, m *discordgo.MessageCreate) {
	if m.Author == nil {
		// reactions are also edit events
		return
	}
	if _, exist := checkCache.Get(fmt.Sprintf("%s%s%s", m.ChannelID, m.Author.ID, m.Content)); exist {
		return
	}
	checkCache.Set(fmt.Sprintf("%s%s%s", m.ChannelID, m.Author.ID, m.Content), true, cache.DefaultExpiration)
	user, err := getUser(m.GuildID, m.Author.ID)
	if err != nil {
		return
	}

	if isUserSafe(user) {
		return
	}

	toRemove := false
	for _, check := range checks {
		if check(s, m) {
			// remove if the check sends true
			toRemove = true
			break
		}
	}

	if toRemove {
		s.ChannelMessageDelete(m.ChannelID, m.ID)
		log.Printf("Removed message from %s aka %s: %s\n", m.Author.ID, m.Author.Username, m.Message.Content)
		notifyUser(m.Author.ID)
	}
}

func checkReaction(s *discordgo.Session, r *discordgo.MessageReactionAdd) {
	user, err := getUser(r.GuildID, r.UserID)
	if err != nil {
		return
	}

	if isUserSafe(user) {
		return
	}

	obj, exists := reactionCache.Get(r.GuildID + r.UserID)
	if !exists {
		obj = 0
	}

	i := obj.(int)
	i++
	if i > 3 {
		s.MessageReactionRemove(r.ChannelID, r.MessageID, r.Emoji.APIName(), r.UserID)
		notifyUserReaction(r.UserID)
	}
	reactionCache.Set(r.GuildID+r.UserID, i, cache.DefaultExpiration)
}

func getUser(gid, uid string) (*discordgo.Member, error) {
	obj, found := userCache.Get(gid + uid)
	if !found {
		user, err := dg.GuildMember(gid, uid)
		if err != nil {
			return nil, err
		}

		userCache.Set(gid+uid, user, cache.DefaultExpiration)
		return user, nil
	}

	return obj.(*discordgo.Member), nil
}

// checks if the user is somebody we should trust
func isUserSafe(m *discordgo.Member) bool {
	safe := true // i trust people on first sight
	for _, role := range m.Roles {
		if role == itfGuestRole {
			safe = false
		}
	}

	return safe
}

func notifyUser(id string) {
	_, hasBeenNotifiedBefore := notifyCache.Get(id)
	if hasBeenNotifiedBefore {
		// limit self spam
		return
	}
	c, err := dg.UserChannelCreate(id)
	if err != nil {
		return
	}

	dg.ChannelMessageSend(c.ID, "Hallo! Ik heb een bericht van je verwijderd omdat het inging tegen de Thomas More ITFactory Discord regels.")
	notifyCache.Add(id, true, cache.DefaultExpiration)
}

func notifyUserReaction(id string) {
	_, hasBeenNotifiedBefore := reactionNotifyCache.Get(id)
	if hasBeenNotifiedBefore {
		// limit self spam
		return
	}
	c, err := dg.UserChannelCreate(id)
	if err != nil {
		return
	}

	dg.ChannelMessageSend(c.ID, "Hallo! Ik heb je reactie van je verwijderd omdat het inging tegen de Thomas More ITFactory Discord regels.")
	reactionNotifyCache.Add(id, true, cache.DefaultExpiration)
}
