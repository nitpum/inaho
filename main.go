package main

import (
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/bwmarrin/discordgo"
	"github.com/samber/lo"
	"gopkg.in/yaml.v3"
)

var (
	Token          string
	ConfigFilepath string
	config         configData
)

type configData struct {
	BotRole struct {
		Enabled bool     `yaml:"enabled"`
		Roles   []string `yaml:"roles"`
	} `yaml:"bot_role"`
	Nickname struct {
		Enabled bool `yaml:"enabled"`
		Members []struct {
			ID     string   `yaml:"id"`
			Prefix []string `yaml:"prefix"`
		} `yaml:"members"`
	} `yaml:"nickname"`
}

func init() {
	flag.StringVar(&Token, "t", "", "Bot Token")
	flag.StringVar(&ConfigFilepath, "c", "config.yaml", "Config path")
	flag.Parse()
}

func main() {

	if Token == "" {
		panic("Token is required")
	}

	c, err := readConfig(ConfigFilepath)
	if err != nil {
		panic(err)
	}
	config = *c

	dg, err := discordgo.New("Bot " + Token)
	if err != nil {
		fmt.Printf("error creating Discord session: %s\n", err)
		return
	}
	defer dg.Close()

	dg.Identify.Intents = discordgo.IntentsGuildMembers | discordgo.IntentGuildInvites

	dg.AddHandler(onGuildMemberAdd)
	dg.AddHandler(onGuildMemberUpdate)
	dg.AddHandler(onInviteCreate)
	// dg.AddHandler(voiceStateUpdate)

	err = dg.Open()
	if err != nil {
		fmt.Printf("error opening connection: %s\n", err)
		return
	}

	fmt.Println("Bot is now running.  Press CTRL-C to exit.")
	sc := make(chan os.Signal, 1)
	signal.Notify(sc, syscall.SIGINT, syscall.SIGTERM, os.Interrupt, os.Kill)
	<-sc
}

func onGuildMemberAdd(s *discordgo.Session, m *discordgo.GuildMemberAdd) {
	if m.User.Bot {
		botMustHaveBotRole(s, m.Member)
	}
}

func onGuildMemberUpdate(session *discordgo.Session, member *discordgo.GuildMemberUpdate) {
	if member.User.Bot {
		botMustHaveBotRole(session, member.Member)
	} else {
		addPrefixToMember(session, member.Member)
	}
}

func onInviteCreate(session *discordgo.Session, inviteContext *discordgo.InviteCreate) {
	if inviteContext.Inviter.Bot {
		return
	}

	if inviteContext.MaxAge > 0 && (inviteContext.MaxUses > 0 && inviteContext.MaxUses <= 10) {
		return
	}

	_, err := session.InviteDelete(inviteContext.Code)
	if err != nil {
		fmt.Printf("failed to delete invite code: %s\n", inviteContext.Code)
		return
	}

	fmt.Printf("Deleted invite code `%s` created by %s(%s)\n", inviteContext.Code, inviteContext.Inviter.Username, inviteContext.Inviter.ID)

	channel, err := session.UserChannelCreate(inviteContext.Inviter.ID)
	if err != nil {
		fmt.Printf("ERROR: Can't dm to inform user about invite code creating\f")
		return
	}

	session.ChannelMessageSend(channel.ID, "Inaho desu~ 任務でありますから I have deleted your invite link。It don't have limit uses or expiration time。 Please create invite link with limit uses 1 - 10 and expiration time onegaishimasu~")
}

// FIXME: This is a hack to get around the fact that the bot can't be deafened
// func voiceStateUpdate(s *discordgo.Session, m *discordgo.VoiceStateUpdate) {
// 	fmt.Printf("Voice state updated")
// 	deafenBot(s, m)
// }

func readConfig(filename string) (*configData, error) {
	buff, err := os.ReadFile(filename)
	if err != nil {
		return nil, err
	}

	c := &configData{}
	err = yaml.Unmarshal(buff, c)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal in %q: %v\n", filename, err)
	}

	return c, nil
}

func memberNickname(s *discordgo.Session, m *discordgo.GuildMemberUpdate) {
	if !config.Nickname.Enabled {
		return
	}

	// Update member nickname to match prefix from config
	for _, member := range config.Nickname.Members {
		if member.ID == m.User.ID {
			match := false
			for _, prefix := range member.Prefix {
				if m.Nick == prefix {
					match = true
					break
				}
			}

			if !match {
				s.GuildMemberNickname(m.GuildID, m.User.ID, member.Prefix[0])
			}
		}
	}
}

func botMustHaveBotRole(s *discordgo.Session, m *discordgo.Member) {
	if !config.BotRole.Enabled {
		return
	}

	if !m.User.Bot {
		return
	}

	for _, role := range config.BotRole.Roles {
		if !lo.Contains(m.Roles, role) {
			err := s.GuildMemberRoleAdd(m.GuildID, m.User.ID, role)
			if err != nil {
				fmt.Printf("error adding role %s to member %s: %s\n", role, m.User.ID, err)
				continue
			}

			fmt.Printf("`%s` was given the `%s` role\n", m.User.Username, role)
		}
	}
}

func deafenBot(s *discordgo.Session, m *discordgo.VoiceStateUpdate) {
	member, err := s.GuildMember(m.GuildID, m.UserID)
	if err != nil {
		fmt.Printf("error getting member %s: %s\n", m.UserID, err)
		return
	}

	if !member.User.Bot {
		return
	}

	if m.VoiceState.ChannelID == "" {
		return
	}

	if m.VoiceState.Deaf {
		return
	}

	channel, err := s.Channel(m.ChannelID)
	if err != nil {
		fmt.Printf("error getting channel %s: %s\n", m.ChannelID, err)
		return
	}

	err = s.GuildMemberDeafen(channel.GuildID, m.UserID, true)
	if err != nil {
		fmt.Printf("error deafening member %s: %s\n", m.UserID, err)
		return
	}

	fmt.Printf("`%s` was deafened\n", member.User.Username)
}

func addPrefixToMember(s *discordgo.Session, m *discordgo.Member) {
	if !config.Nickname.Enabled {
		return
	}

	if m.User.Bot {
		return
	}

	for _, member := range config.Nickname.Members {
		if member.ID == m.User.ID {
			nickname := strings.TrimSpace(m.Nick)

			validNickname := false
			for _, prefix := range member.Prefix {
				if nickname == prefix || strings.HasPrefix(nickname, prefix) {
					validNickname = true
					break
				}
			}

			if validNickname {
				break
			}

			err := s.GuildMemberNickname(m.GuildID, m.User.ID, member.Prefix[0]+m.Nick)
			if err != nil {
				fmt.Printf("error adding prefix to member %s: %s\n", m.User.ID, err)
				break
			}

			fmt.Printf("%s was added missing prefix nickname %s\n", m.User.Username, member.Prefix[0])

			break
		}
	}
}
