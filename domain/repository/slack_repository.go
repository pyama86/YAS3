package repository

import (
	"context"
	"fmt"
	"log/slog"
	"regexp"
	"strings"
	"time"

	"github.com/Songmu/retry"
	ttlcache "github.com/jellydator/ttlcache/v3"
	"github.com/slack-go/slack"
)

var ErrSlackNotFound = fmt.Errorf("not found")

type SlackRepositoryer interface {
	GetUserByID(id string) (*slack.User, error)
	GetSlackID(name string) (string, error)
	GetMemberIDs(name string) ([]string, error)
	GetChannelByName(name string) (*slack.Channel, error)
	GetChannelByID(channelID string) (*slack.Channel, error)
	PostMessage(channelID string, opts ...slack.MsgOption)
	UpdateMessage(channelID, ts string, opts ...slack.MsgOption)
	DeleteMessage(channelID, ts string)
	OpenView(triggerID string, view slack.ModalViewRequest) error
	CreateConversation(params slack.CreateConversationParams) (*slack.Channel, error)
	SetTopicOfConversation(channelID, topic string) error
	InviteUsersToConversation(channelID string, users ...string) error
	GetPinnedMessages(channelID string) ([]slack.Message, error)
	GetUserPreferredName(user *slack.User) string
	UploadFile(workspackeURL, userID, channelID, filename, title, content string) (string, error)
	FlushChannelCache()
}

type SlackRepository struct {
	client             *slack.Client
	channelsCache      *ttlcache.Cache[string, []slack.Channel]
	usersCache         *ttlcache.Cache[string, []slack.User]
	groupsCache        *ttlcache.Cache[string, []slack.UserGroup]
	userNameCache      *ttlcache.Cache[string, *slack.User]
	userGroupNameCache *ttlcache.Cache[string, *slack.UserGroup]
}

func NewSlackRepository(client *slack.Client) *SlackRepository {

	r := &SlackRepository{
		client:             client,
		channelsCache:      ttlcache.New(ttlcache.WithTTL[string, []slack.Channel](time.Hour)),
		usersCache:         ttlcache.New(ttlcache.WithTTL[string, []slack.User](time.Hour)),
		groupsCache:        ttlcache.New(ttlcache.WithTTL[string, []slack.UserGroup](time.Hour)),
		userNameCache:      ttlcache.New(ttlcache.WithTTL[string, *slack.User](time.Hour)),
		userGroupNameCache: ttlcache.New(ttlcache.WithTTL[string, *slack.UserGroup](time.Hour)),
	}
	go r.channelsCache.Start()
	go r.usersCache.Start()
	go r.groupsCache.Start()
	go r.userNameCache.Start()
	go r.userGroupNameCache.Start()

	go func() {
		_, err := r.getChannels()
		if err != nil {
			slog.Error("Failed to get channels", slog.Any("err", err))
		}
		slog.Info("Channels cache initialized")
		_, err = r.getUsers()
		if err != nil {
			slog.Error("Failed to get users", slog.Any("err", err))
		}
		slog.Info("Users cache initialized")
		_, err = r.getUserGroups()
		if err != nil {
			slog.Error("Failed to get user groups", slog.Any("err", err))
		}
		slog.Info("User groups cache initialized")
	}()
	// 失効時は自動で更新する
	r.channelsCache.OnEviction(func(ctx context.Context, _ ttlcache.EvictionReason, _ *ttlcache.Item[string, []slack.Channel]) {
		slog.Info("Refreshing channels cache")
		_, err := r.getChannels()
		if err != nil {
			slog.Error("Failed to refresh channels cache", slog.Any("err", err))
		}
	})

	r.usersCache.OnEviction(func(ctx context.Context, _ ttlcache.EvictionReason, _ *ttlcache.Item[string, []slack.User]) {
		slog.Info("Refreshing users cache")
		_, err := r.getUsers()
		if err != nil {
			slog.Error("Failed to refresh users cache", slog.Any("err", err))
		}
	})

	r.groupsCache.OnEviction(func(ctx context.Context, _ ttlcache.EvictionReason, _ *ttlcache.Item[string, []slack.UserGroup]) {
		slog.Info("Refreshing groups cache")
		_, err := r.getUserGroups()
		if err != nil {
			slog.Error("Failed to refresh groups cache", slog.Any("err", err))
		}
	})
	return r
}

func (h *SlackRepository) FlushChannelCache() {
	h.channelsCache.DeleteAll()
}

func (h *SlackRepository) GetUserByID(id string) (*slack.User, error) {
	users, err := h.getUsers()
	if err != nil {
		return nil, err
	}
	for _, u := range users {
		if u.ID == id {
			return &u, nil
		}
	}
	return nil, ErrSlackNotFound
}

func (h *SlackRepository) GetSlackID(name string) (string, error) {
	users, err := h.getUsers()
	if err != nil {
		return "", err
	}
	groups, err := h.getUserGroups()
	if err != nil {
		return "", err
	}

	for _, u := range users {
		if strings.EqualFold(u.Name, name) ||
			strings.EqualFold(u.Profile.DisplayName, name) ||
			strings.EqualFold(u.RealName, name) ||
			strings.EqualFold(u.Profile.RealName, name) {
			return u.ID, nil
		}
	}
	for _, g := range groups {
		if strings.EqualFold(g.Handle, name) ||
			strings.EqualFold(g.Name, name) {
			return g.ID, nil
		}
	}

	return "", ErrSlackNotFound
}

func (h *SlackRepository) getUsers() ([]slack.User, error) {
	cacheKey := "users"
	if users := h.usersCache.Get(cacheKey); users != nil {
		return users.Value(), nil
	}
	users, err := h.client.GetUsers()
	if err != nil {
		return nil, err
	}
	h.usersCache.Set(cacheKey, users, ttlcache.DefaultTTL)

	for _, u := range users {
		if u.Name != "" {
			h.userNameCache.Set(u.Name, &u, ttlcache.DefaultTTL)
		}
	}

	return users, nil
}

func (h *SlackRepository) getUserGroups() ([]slack.UserGroup, error) {
	cacheKey := "user_groups"
	if groups := h.groupsCache.Get(cacheKey); groups != nil {
		return groups.Value(), nil
	}
	groups, err := h.client.GetUserGroups(
		slack.GetUserGroupsOptionIncludeUsers(true),
	)
	if err != nil {
		return nil, err
	}
	h.groupsCache.Set(cacheKey, groups, ttlcache.DefaultTTL)

	for _, g := range groups {
		if g.Handle != "" {
			h.userGroupNameCache.Set(g.Handle, &g, ttlcache.DefaultTTL)
		}
		if g.Name != "" {
			h.userGroupNameCache.Set(g.Name, &g, ttlcache.DefaultTTL)
		}
	}

	return groups, nil
}

func (h *SlackRepository) getChannels() ([]slack.Channel, error) {
	cacheKey := "channels"
	if channels := h.channelsCache.Get(cacheKey); channels != nil {
		return channels.Value(), nil
	}
	nextCursor := ""
	channels := make([]slack.Channel, 0)
	for {
		cs, next, err := h.client.GetConversations(&slack.GetConversationsParameters{
			Limit:           1000,
			Cursor:          nextCursor,
			ExcludeArchived: false,
		})
		if err != nil {
			return nil, err
		}
		channels = append(channels, cs...)
		if next == "" {
			break
		}
		nextCursor = next
	}

	h.channelsCache.Set(cacheKey, channels, ttlcache.DefaultTTL)
	return channels, nil
}

func (h *SlackRepository) GetChannelByID(channelID string) (*slack.Channel, error) {
	channels, err := h.getChannels()
	if err != nil {
		return nil, err
	}
	for _, c := range channels {
		if c.ID == channelID {
			return &c, nil
		}
	}
	return nil, ErrSlackNotFound
}

func (h *SlackRepository) GetChannelByName(name string) (*slack.Channel, error) {
	channels, err := h.getChannels()
	if err != nil {
		return nil, err
	}
	for _, c := range channels {
		if c.Name == strings.TrimPrefix(name, "#") {
			return &c, nil
		}
	}
	return nil, nil
}

func (h *SlackRepository) PostMessage(channelID string, opts ...slack.MsgOption) {
	go func() {
		err := retry.Retry(10, 3*time.Second, func() error {
			_, _, err := h.client.PostMessage(channelID, opts...)
			if err != nil {
				slog.Warn("PostMessage", slog.Any("channelID", channelID), slog.Any("err", err))
			}
			return err
		})
		if err != nil {
			slog.Error("Failed to PostMessage", slog.Any("err", err))
		}
	}()
}

func (h *SlackRepository) UpdateMessage(channelID, ts string, opts ...slack.MsgOption) {
	go func() {
		err := retry.Retry(10, 3*time.Second, func() error {
			_, _, _, err := h.client.UpdateMessage(channelID, ts, opts...)
			if err != nil {
				slog.Warn("UpdateMessage", slog.Any("channelID", channelID), slog.Any("ts", ts), slog.Any("err", err))
			}
			return err
		})
		if err != nil {
			slog.Error("Failed to UpdateMessage", slog.Any("err", err))
		}
	}()
}

func (h *SlackRepository) DeleteMessage(channelID, ts string) {
	go func() {
		err := retry.Retry(10, 3*time.Second, func() error {
			_, _, err := h.client.DeleteMessage(channelID, ts)
			if err != nil {
				slog.Warn("DeleteMessage", slog.Any("channelID", channelID), slog.Any("ts", ts), slog.Any("err", err))
			}
			return err
		})
		if err != nil {
			slog.Error("Failed to DeleteMessage", slog.Any("err", err))
		}
	}()
}

func (h *SlackRepository) GetMemberIDs(name string) ([]string, error) {
	if h.userNameCache.Get(name) != nil {
		return []string{h.userNameCache.Get(name).Value().ID}, nil
	}
	if h.userGroupNameCache.Get(name) != nil {
		return h.userGroupNameCache.Get(name).Value().Users, nil
	}

	users, err := h.getUsers()
	if err != nil {
		return nil, err
	}
	groups, err := h.getUserGroups()
	if err != nil {
		return nil, err
	}

	for _, u := range users {
		if strings.EqualFold(u.Name, name) ||
			strings.EqualFold(u.Profile.DisplayName, name) ||
			strings.EqualFold(u.RealName, name) ||
			strings.EqualFold(u.Profile.RealName, name) {
			return []string{u.ID}, nil
		}

	}
	for _, g := range groups {
		if strings.EqualFold(g.Handle, name) ||
			strings.EqualFold(g.Name, name) {
			return g.Users, nil
		}
	}
	return nil, ErrSlackNotFound
}

func (h *SlackRepository) OpenView(triggerID string, view slack.ModalViewRequest) error {
	err := retry.Retry(10, 3*time.Second, func() error {
		_, err := h.client.OpenView(triggerID, view)
		if err != nil {
			slog.Warn("OpenView", slog.Any("triggerID", triggerID), slog.Any("err", err))
		}
		return err
	})
	if err != nil {
		slog.Error("Failed to OpenView", slog.Any("err", err))
	}
	return err
}

func (h *SlackRepository) CreateConversation(params slack.CreateConversationParams) (*slack.Channel, error) {
	var channel *slack.Channel
	err := retry.Retry(3, 3*time.Second, func() error {
		var err error
		channel, err = h.client.CreateConversation(params)
		if err != nil {
			slog.Warn("CreateConversation", slog.Any("params", params), slog.Any("err", err))
		}
		return err
	})
	if err != nil {
		slog.Error("Failed to CreateConversation", slog.Any("err", err))
	}
	return channel, err
}

func (h *SlackRepository) SetTopicOfConversation(channelID, topic string) error {
	err := retry.Retry(10, 3*time.Second, func() error {
		_, err := h.client.SetTopicOfConversation(channelID, topic)
		if err != nil {
			slog.Warn("SetTopicOfConversation", slog.Any("channelID", channelID), slog.Any("topic", topic), slog.Any("err", err))
		}
		return err
	})
	if err != nil {
		slog.Error("Failed to SetTopicOfConversation", slog.Any("err", err))
	}
	return err
}

func (h *SlackRepository) InviteUsersToConversation(channelID string, users ...string) error {
	err := retry.Retry(10, 3*time.Second, func() error {
		_, err := h.client.InviteUsersToConversation(channelID, users...)
		if err != nil {
			slog.Warn("InviteUsersToConversation", slog.Any("channelID", channelID), slog.Any("users", users), slog.Any("err", err))
		}
		return err
	})
	if err != nil {
		slog.Error("Failed to InviteUsersToConversation", slog.Any("err", err))
	}
	return err
}

// ピンが付いているメッセージを取得
func (h *SlackRepository) GetPinnedMessages(channelID string) ([]slack.Message, error) {
	items, _, err := h.client.ListPins(channelID)
	if err != nil {
		return nil, err
	}

	var messages []slack.Message
	re := regexp.MustCompile(`<@([A-Z0-9]+)>`)
	cache := make(map[string]string)

	for _, item := range items {
		if item.Message == nil {
			continue
		}
		msg := *item.Message
		if msg.Text != "" {
			newText := re.ReplaceAllStringFunc(msg.Text, func(match string) string {
				submatches := re.FindStringSubmatch(match)
				if len(submatches) < 2 {
					return match
				}
				userID := submatches[1]
				if realName, ok := cache[userID]; ok {
					return "@" + realName
				}
				user, err := h.GetUserByID(userID)
				if err != nil {
					return match
				}
				realName := h.GetUserPreferredName(user)
				cache[userID] = realName
				return "@" + realName
			})
			msg.Text = newText
		}
		messages = append(messages, msg)
	}

	return messages, nil
}
func (h *SlackRepository) GetUserPreferredName(user *slack.User) string {
	if user.Profile.DisplayName != "" {
		return user.Profile.DisplayName
	}
	if user.RealName != "" {
		return user.RealName
	}
	return user.Name
}
func (h *SlackRepository) UploadFile(workspackeURL, userID, channelID, filename, title, content string) (string, error) {
	f, err := h.client.UploadFileV2(slack.UploadFileV2Parameters{
		Channel:  channelID,
		Filename: filename,
		Title:    title,
		AltTxt:   title,
		Content:  content,
		FileSize: len(content),
	})
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%s/files/%s/%s", workspackeURL, userID, f.ID), nil
}
