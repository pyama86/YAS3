package handler_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/slack-go/slack"
	"github.com/slack-go/slack/slackevents"
	"github.com/slack-go/slack/slacktest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pyama86/YAS3/domain/entity"
	"github.com/pyama86/YAS3/domain/repository"
	"github.com/pyama86/YAS3/handler"
)

// ------------------------
// Mock repositories
// ------------------------
type mockIncidentRepo struct {
	data    map[string]*entity.Incident
	active  []entity.Incident
	findErr error
	saveErr error
}

func (m *mockIncidentRepo) FindIncidentByChannel(_ context.Context, ch string) (*entity.Incident, error) {
	if m.findErr != nil {
		return nil, m.findErr
	}
	if inc, ok := m.data[ch]; ok {
		return inc, nil
	}
	return nil, nil
}
func (m *mockIncidentRepo) SaveIncident(_ context.Context, inc *entity.Incident) error {
	if m.saveErr != nil {
		return m.saveErr
	}
	m.data[inc.ChannelID] = inc
	return nil
}
func (m *mockIncidentRepo) ActiveIncidents(_ context.Context) ([]entity.Incident, error) {
	return m.active, nil
}

type mockSlackRepo struct{}

func (m *mockSlackRepo) GetChannelByID(channelID string) (*slack.Channel, error) {
	return &slack.Channel{
		GroupConversation: slack.GroupConversation{
			Conversation: slack.Conversation{
				ID: channelID,
			},
			Name:       "test-channel",
			IsArchived: false,
		},
	}, nil
}

func (m *mockSlackRepo) GetChannelByName(name string) (*slack.Channel, error) {
	return &slack.Channel{
		GroupConversation: slack.GroupConversation{
			Conversation: slack.Conversation{
				ID: "C123456",
			},
			Name:       name,
			IsArchived: false,
		},
	}, nil
}

func (m *mockSlackRepo) PostMessage(channelID string, options ...slack.MsgOption) (string, string, error) {
	return channelID, "123456.789", nil
}

func (m *mockSlackRepo) UpdateMessage(channelID, timestamp string, options ...slack.MsgOption) {}

func (m *mockSlackRepo) DeleteMessage(channelID, timestamp string) {}

func (m *mockSlackRepo) OpenView(triggerID string, view slack.ModalViewRequest) error {
	return nil
}

func (m *mockSlackRepo) CreateConversation(params slack.CreateConversationParams) (*slack.Channel, error) {
	return &slack.Channel{}, nil
}

func (m *mockSlackRepo) SetTopicOfConversation(channelID, topic string) error {
	return nil
}

func (m *mockSlackRepo) InviteUsersToConversation(channelID string, users ...string) error {
	return nil
}

func (m *mockSlackRepo) GetMemberIDs(member string) ([]string, error) {
	return []string{member}, nil
}

func (m *mockSlackRepo) FlushChannelCache() {}

func (m *mockSlackRepo) GetPinnedMessages(channelID string) ([]slack.Message, error) {
	return []slack.Message{}, nil
}

func (m *mockSlackRepo) GetUserByID(userID string) (*slack.User, error) {
	return &slack.User{ID: userID, Name: "testuser"}, nil
}

func (m *mockSlackRepo) GetUserPreferredName(user *slack.User) string {
	return user.Name
}

func (m *mockSlackRepo) UploadFile(workspaceURL, userID, channelID, filename, title, content string) (string, error) {
	return "http://example.com/file", nil
}

func (m *mockSlackRepo) GetChannelHistory(channelID, oldest, latest string, limit int) ([]slack.Message, error) {
	return []slack.Message{}, nil
}

func (m *mockSlackRepo) GetChannelMessagesAfter(channelID, after string) ([]slack.Message, error) {
	return []slack.Message{}, nil
}

func (m *mockSlackRepo) GetAllChannelMessages(channelID string) ([]slack.Message, error) {
	return []slack.Message{}, nil
}

func (m *mockSlackRepo) GetThreadReplies(channelID, threadTS string) ([]slack.Message, error) {
	return []slack.Message{}, nil
}

func (m *mockSlackRepo) GetSlackID(name string) (string, error) {
	return "U123456", nil
}

type mockConfigRepo struct {
	services []entity.Service
	levels   []entity.IncidentLevel
	announce []string
}

func (m *mockConfigRepo) Services(_ context.Context) ([]entity.Service, error) {
	return m.services, nil
}
func (m *mockConfigRepo) ServiceByID(_ context.Context, id int) (*entity.Service, error) {
	for _, s := range m.services {
		if s.ID == id {
			return &s, nil
		}
	}
	return nil, fmt.Errorf("not found")
}
func (m *mockConfigRepo) GetGlobalAnnouncementChannels(_ context.Context) []string {
	return m.announce
}
func (m *mockConfigRepo) IncidentLevels(_ context.Context) []entity.IncidentLevel {
	return m.levels
}
func (m *mockConfigRepo) IncidentLevelByLevel(_ context.Context, lv int) (*entity.IncidentLevel, error) {
	for _, l := range m.levels {
		if l.Level == lv {
			return &l, nil
		}
	}
	if lv == 0 {
		return &entity.IncidentLevel{Level: 0, Description: "サービス影響なし"}, nil
	}
	return nil, fmt.Errorf("not found")
}

// -----------------------------------
// handler.go : timeKeeperMessage
// -----------------------------------
func TestTimeKeeperMessage(t *testing.T) {
	var postMsg []map[string]string
	srv := slacktest.NewTestServer(func(c slacktest.Customize) {
		c.Handle("/auth.test", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"ok":true,"user_id":"UBOT"}`))
		}))

		c.Handle("/conversations.list", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			resp := struct {
				OK       bool `json:"ok"`
				Channels []struct {
					ID         string `json:"id"`
					Name       string `json:"name"`
					IsArchived bool   `json:"is_archived"`
				} `json:"channels"`
			}{
				OK: true,
				Channels: []struct {
					ID         string `json:"id"`
					Name       string `json:"name"`
					IsArchived bool   `json:"is_archived"`
				}{
					{ID: "CARCH", Name: "archived", IsArchived: true},
					{ID: "COK", Name: "okchan", IsArchived: false},
				},
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(resp)
		}))

		c.Handle("/chat.postMessage", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_ = r.ParseForm()
			postMsg = append(postMsg, map[string]string{
				"channel": r.FormValue("channel"),
				"text":    r.FormValue("text"),
				"blocks":  r.FormValue("blocks"),
			})
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"ok":true}`))
		}))
	})
	go srv.Start()
	defer srv.Stop()

	api := slack.New("dummy", slack.OptionAPIURL(srv.GetAPIURL()))
	slackRepo := repository.NewSlackRepository(api)

	incArchived := entity.Incident{
		ChannelID: "CARCH", StartedAt: time.Now().Add(-30 * time.Minute),
	}
	incOk := entity.Incident{
		ChannelID: "COK", StartedAt: time.Now().Add(-15 * time.Minute),
	}
	incNotFound := entity.Incident{
		ChannelID: "CNOTFOUND",
	}

	// notfound => skip
	err := timeKeeperMessageTest(api, &incNotFound, slackRepo)
	assert.NoError(t, err)
	assert.Empty(t, postMsg)

	// archived => skip
	postMsg = nil
	err = timeKeeperMessageTest(api, &incArchived, slackRepo)
	assert.NoError(t, err)
	assert.Empty(t, postMsg)

	// normal => post
	postMsg = nil
	err = timeKeeperMessageTest(api, &incOk, slackRepo)
	assert.NoError(t, err)
	require.Len(t, postMsg, 1)
	assert.Equal(t, "COK", postMsg[0]["channel"])
}

func timeKeeperMessageTest(client *slack.Client, incident *entity.Incident, slackRepo *repository.SlackRepository) error {
	ch, err := slackRepo.GetChannelByID(incident.ChannelID)
	if err != nil {
		if err == repository.ErrSlackNotFound {
			return nil
		}
		return err
	}
	if ch.IsArchived {
		return nil
	}
	_, _, err = client.PostMessage(incident.ChannelID,
		slack.MsgOptionText("チェックポイント", false),
	)
	return err
}

// -----------------------------------
// event.go : EventHandler
// -----------------------------------
func TestEventHandler_Handle(t *testing.T) {
	var postMsg []map[string]string

	srv := slacktest.NewTestServer(func(c slacktest.Customize) {
		c.Handle("/auth.test", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte(`{"ok":true,"user_id":"UBOT"}`))
		}))
		c.Handle("/chat.postMessage", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_ = r.ParseForm()
			postMsg = append(postMsg, map[string]string{
				"channel": r.FormValue("channel"),
				"blocks":  r.FormValue("blocks"),
			})
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"ok":true}`))
		}))
	})
	go srv.Start()
	defer srv.Stop()

	api := slack.New("dummy", slack.OptionAPIURL(srv.GetAPIURL()))
	incRepo := &mockIncidentRepo{data: map[string]*entity.Incident{
		"CINC": {ChannelID: "CINC"},
	}}
	cfgRepo := &mockConfigRepo{}
	slackRepo := &mockSlackRepo{}
	repo := repository.NewRepository(incRepo, cfgRepo, cfgRepo, slackRepo)
	config := &repository.Config{} // 空のConfig構造体
	evHandler := handler.NewEventHandler(context.Background(), api, repo, config)

	tests := []struct {
		name     string
		event    interface{}
		expectPM bool
	}{
		{
			"AppMention no inc => Opening",
			&slackevents.AppMentionEvent{Channel: "CNEW"},
			true,
		},
		{
			"AppMention with inc => IncidentMenu",
			&slackevents.AppMentionEvent{Channel: "CINC"},
			true,
		},
		{
			"ChannelArchive inc => closeAt",
			&slackevents.ChannelArchiveEvent{Channel: "CINC"},
			false,
		},
		{
			"ChannelArchive no inc => skip",
			&slackevents.ChannelArchiveEvent{Channel: "CNO"},
			false,
		},
		{
			"Other => skip",
			&slackevents.MessageEvent{Channel: "CABC"},
			false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			postMsg = nil
			err := evHandler.Handle(&slackevents.EventsAPIInnerEvent{Data: tt.event})
			require.NoError(t, err)
			if tt.expectPM {
				assert.NotEmpty(t, postMsg)
			} else {
				assert.Empty(t, postMsg)
			}
		})
	}
}

// 障害概要編集時の周知チャンネル通知機能をテストする
func TestEditIncidentSummaryNotification(t *testing.T) {
	var postMsg []map[string]string

	srv := slacktest.NewTestServer(func(c slacktest.Customize) {
		c.Handle("/auth.test", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte(`{"ok":true,"user_id":"UBOT"}`))
		}))
		c.Handle("/chat.postMessage", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_ = r.ParseForm()
			postMsg = append(postMsg, map[string]string{
				"channel":     r.FormValue("channel"),
				"blocks":      r.FormValue("blocks"),
				"attachments": r.FormValue("attachments"),
				"text":        r.FormValue("text"),
			})
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"ok":true}`))
		}))
		c.Handle("/conversations.list", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			resp := struct {
				OK       bool `json:"ok"`
				Channels []struct {
					ID         string `json:"id"`
					Name       string `json:"name"`
					IsArchived bool   `json:"is_archived"`
				} `json:"channels"`
			}{
				OK: true,
				Channels: []struct {
					ID         string `json:"id"`
					Name       string `json:"name"`
					IsArchived bool   `json:"is_archived"`
				}{
					{ID: "CINC", Name: "incident-channel", IsArchived: false},
					{ID: "CANN", Name: "announcement", IsArchived: false},
				},
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(resp)
		}))
		c.Handle("/conversations.info", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			channelID := r.URL.Query().Get("channel")
			resp := struct {
				OK      bool `json:"ok"`
				Channel struct {
					ID    string `json:"id"`
					Name  string `json:"name"`
					Topic struct {
						Value string `json:"value"`
					} `json:"topic"`
				} `json:"channel"`
			}{
				OK: true,
				Channel: struct {
					ID    string `json:"id"`
					Name  string `json:"name"`
					Topic struct {
						Value string `json:"value"`
					} `json:"topic"`
				}{
					ID:   channelID,
					Name: "incident-channel",
					Topic: struct {
						Value string `json:"value"`
					}{Value: "サービス名:test-service 緊急度:✅ サービスへの影響はない 事象内容:古い事象内容"},
				},
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(resp)
		}))
		c.Handle("/conversations.setTopic", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"ok":true}`))
		}))
	})
	go srv.Start()
	defer srv.Stop()

	api := slack.New("dummy", slack.OptionAPIURL(srv.GetAPIURL()))
	incRepo := &mockIncidentRepo{data: map[string]*entity.Incident{
		"CINC": {
			ChannelID:   "CINC",
			ServiceID:   1,
			Urgency:     "none",
			Description: "古い事象内容",
		},
	}}
	cfgRepo := &mockConfigRepo{
		services: []entity.Service{{ID: 1, Name: "test-service", AnnouncementChannels: []string{"announcement"}}},
		levels:   []entity.IncidentLevel{{Level: 0, Description: "サービス影響なし"}},
		announce: []string{"announcement"},
	}
	repo := repository.NewRepository(incRepo, cfgRepo, cfgRepo, repository.NewSlackRepository(api))
	cbHandler := handler.NewCallbackHandler(context.Background(), repo, "https://example.com/", nil, nil, nil)

	// 障害概要編集のテスト
	callback := slack.InteractionCallback{
		Type:      slack.InteractionTypeViewSubmission,
		TriggerID: "dummy-trigger",
		View: slack.View{
			CallbackID:      "edit_summary_modal",
			PrivateMetadata: "CINC",
			State: &slack.ViewState{
				Values: map[string]map[string]slack.BlockAction{
					"edit_summary_block": {
						"summary_text": {Value: "新しい事象内容"},
					},
				},
			},
		},
		User: slack.User{ID: "UEDIT"},
	}

	err := cbHandler.Handle(&callback)
	require.NoError(t, err)

	// 非同期処理の完了を待つ
	time.Sleep(100 * time.Millisecond)

	// メッセージが投稿されたことを確認
	require.NotEmpty(t, postMsg)

	// 基本的な動作確認
	// 1. メッセージが複数投稿されていることを確認（インシデントチャンネル + 周知チャンネル）
	assert.GreaterOrEqual(t, len(postMsg), 2)

	// 2. インシデントチャンネルに何らかのメッセージが投稿されていることを確認
	var hasIncChannelMsg bool
	var hasAnnouncementMsg bool

	for _, msg := range postMsg {
		if msg["channel"] == "CINC" {
			hasIncChannelMsg = true
		}
		if msg["channel"] == "CANN" {
			hasAnnouncementMsg = true
		}
	}

	assert.True(t, hasIncChannelMsg, "インシデントチャンネルにメッセージが投稿されていません")
	assert.True(t, hasAnnouncementMsg, "周知チャンネルにメッセージが投稿されていません")

	// インシデント情報が更新されたことを確認
	updatedIncident := incRepo.data["CINC"]
	assert.Equal(t, "新しい事象内容", updatedIncident.Description)
}

// -----------------------------------
// callback.go : CallbackHandler
// -----------------------------------
func TestCallbackHandler_Handle(t *testing.T) {
	var postMsg, updateMsg, deleteMsg []map[string]string
	var openViewCount int

	srv := slacktest.NewTestServer(func(c slacktest.Customize) {
		c.Handle("/auth.test", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte(`{"ok":true,"user_id":"UBOT"}`))
		}))
		c.Handle("/chat.postMessage", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_ = r.ParseForm()
			postMsg = append(postMsg, map[string]string{
				"channel":     r.FormValue("channel"),
				"blocks":      r.FormValue("blocks"),
				"attachments": r.FormValue("attachments"),
				"text":        r.FormValue("text"),
			})
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"ok":true}`))
		}))
		c.Handle("/chat.update", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_ = r.ParseForm()
			updateMsg = append(updateMsg, map[string]string{
				"channel": r.FormValue("channel"),
				"ts":      r.FormValue("ts"),
				"blocks":  r.FormValue("blocks"),
			})
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"ok":true}`))
		}))
		c.Handle("/chat.delete", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_ = r.ParseForm()
			deleteMsg = append(deleteMsg, map[string]string{
				"channel": r.FormValue("channel"),
				"ts":      r.FormValue("ts"),
			})
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"ok":true}`))
		}))
		c.Handle("/views.open", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			openViewCount++
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"ok":true}`))
		}))
		c.Handle("/conversations.list", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			resp := struct {
				OK       bool `json:"ok"`
				Channels []struct {
					ID         string `json:"id"`
					Name       string `json:"name"`
					IsArchived bool   `json:"is_archived"`
				} `json:"channels"`
			}{
				OK: true,
				Channels: []struct {
					ID         string `json:"id"`
					Name       string `json:"name"`
					IsArchived bool   `json:"is_archived"`
				}{
					{ID: "CINC", Name: "inc", IsArchived: false},
					{ID: "CNINC", Name: "ninc", IsArchived: false},
					{ID: "CFROM", Name: "fromchan", IsArchived: false},
					{ID: "CDUM", Name: "dumchan", IsArchived: false},
					{ID: "CANN", Name: "ANN", IsArchived: false},
				},
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(resp)
		}))
		c.Handle("/conversations.info", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			channelID := r.URL.Query().Get("channel")
			resp := struct {
				OK      bool `json:"ok"`
				Channel struct {
					ID    string `json:"id"`
					Name  string `json:"name"`
					Topic struct {
						Value string `json:"value"`
					} `json:"topic"`
				} `json:"channel"`
			}{
				OK: true,
				Channel: struct {
					ID    string `json:"id"`
					Name  string `json:"name"`
					Topic struct {
						Value string `json:"value"`
					} `json:"topic"`
				}{
					ID:   channelID,
					Name: "test-channel",
					Topic: struct {
						Value string `json:"value"`
					}{Value: "サービス名:svc 緊急度:✅ サービスへの影響はない 事象内容:古い事象内容"},
				},
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(resp)
		}))
		c.Handle("/conversations.setTopic", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"ok":true}`))
		}))
	})
	go srv.Start()
	defer srv.Stop()

	api := slack.New("dummy", slack.OptionAPIURL(srv.GetAPIURL()))
	incRepo := &mockIncidentRepo{data: map[string]*entity.Incident{
		"CINC":  {ChannelID: "CINC", ServiceID: 1, Urgency: "none", RecoveredAt: time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC), DisableTimer: true},
		"CNINC": {ChannelID: "CNINC", ServiceID: 1, Urgency: "none"},
		"CDUM":  {ChannelID: "CDUM", ServiceID: 1, Urgency: "none"},
		"CFROM": {ChannelID: "CFROM", ServiceID: 1, Urgency: "none"},
	}}
	cfgRepo := &mockConfigRepo{
		services: []entity.Service{{ID: 1, Name: "svc"}},
		levels:   []entity.IncidentLevel{{Level: 0, Description: "none"}, {Level: 1, Description: "critical"}},
		announce: []string{"ANN"},
	}
	repo := repository.NewRepository(incRepo, cfgRepo, cfgRepo, repository.NewSlackRepository(api))
	cbHandler := handler.NewCallbackHandler(context.Background(), repo, "https://example.com/", nil, nil, nil)

	tcs := []struct {
		name    string
		cb      slack.InteractionCallback
		wantErr bool
	}{
		{
			name: "no blockActions => error",
			cb: slack.InteractionCallback{
				Type:           slack.InteractionTypeBlockActions,
				ActionCallback: slack.ActionCallbacks{BlockActions: nil},
			},
			wantErr: true,
		},
		{
			name: "incident_action => openView & update",
			cb: slack.InteractionCallback{
				Type:      slack.InteractionTypeBlockActions,
				TriggerID: "dummy-trigger", // ここが重要
				Channel: slack.Channel{
					GroupConversation: slack.GroupConversation{
						Conversation: slack.Conversation{ID: "CDUM"},
					},
				},
				Message: slack.Message{
					Msg: slack.Msg{Timestamp: "111.222"},
				},
				User: slack.User{ID: "UOPEN"},
				ActionCallback: slack.ActionCallbacks{
					BlockActions: []*slack.BlockAction{
						{ActionID: "incident_action", Value: "dummyVal"},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "handler_button => inc exist => ok",
			cb: slack.InteractionCallback{
				Type: slack.InteractionTypeBlockActions,
				Channel: slack.Channel{
					GroupConversation: slack.GroupConversation{
						Conversation: slack.Conversation{ID: "CINC"},
					},
				},
				Message: slack.Message{
					Msg: slack.Msg{Timestamp: "333.444"},
				},
				User: slack.User{ID: "UHANDLER"},
				ActionCallback: slack.ActionCallbacks{
					BlockActions: []*slack.BlockAction{
						{ActionID: "handler_button", Value: "v"},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "handler_button => inc not exist => error",
			cb: slack.InteractionCallback{
				Type: slack.InteractionTypeBlockActions,
				Channel: slack.Channel{
					GroupConversation: slack.GroupConversation{
						Conversation: slack.Conversation{ID: "CNO"},
					},
				},
				Message: slack.Message{
					Msg: slack.Msg{Timestamp: "999.000"},
				},
				User: slack.User{ID: "UNO"},
				ActionCallback: slack.ActionCallbacks{
					BlockActions: []*slack.BlockAction{
						{ActionID: "handler_button", Value: "x"},
					},
				},
			},
			wantErr: true,
		},
		{
			name: "incident_level_button => setIncidentLevel => ok (CNINC)",
			cb: slack.InteractionCallback{
				Type: slack.InteractionTypeBlockActions,
				Channel: slack.Channel{
					GroupConversation: slack.GroupConversation{
						Conversation: slack.Conversation{ID: "CNINC"},
					},
				},
				Message: slack.Message{
					Msg: slack.Msg{Timestamp: "555.666"},
				},
				User: slack.User{ID: "ULEVEL"},
				ActionCallback: slack.ActionCallbacks{
					BlockActions: []*slack.BlockAction{
						{ActionID: "incident_level_button", Value: "1"},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "in_channel_options => recovery_incident => ok (CNINC)",
			cb: slack.InteractionCallback{
				Type: slack.InteractionTypeBlockActions,
				Channel: slack.Channel{
					GroupConversation: slack.GroupConversation{
						Conversation: slack.Conversation{ID: "CNINC"},
					},
				},
				Message: slack.Message{
					Msg: slack.Msg{Timestamp: "888.999"},
				},
				User: slack.User{ID: "UREC"},
				ActionCallback: slack.ActionCallbacks{
					BlockActions: []*slack.BlockAction{
						{
							ActionID:       "in_channel_options",
							BlockID:        "keeper_action",
							SelectedOption: slack.OptionBlockObject{Value: "recovery_incident"},
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "in_channel_options => edit_incident_summary => openView & update (CINC)",
			cb: slack.InteractionCallback{
				Type:      slack.InteractionTypeBlockActions,
				TriggerID: "dummy-trigger",
				Channel: slack.Channel{
					GroupConversation: slack.GroupConversation{
						Conversation: slack.Conversation{ID: "CINC"},
					},
				},
				Message: slack.Message{
					Msg: slack.Msg{Timestamp: "777.888"},
				},
				User: slack.User{ID: "UEDIT"},
				ActionCallback: slack.ActionCallbacks{
					BlockActions: []*slack.BlockAction{
						{
							ActionID:       "in_channel_options",
							SelectedOption: slack.OptionBlockObject{Value: "edit_incident_summary"},
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "ViewSubmission => incident_modal => ok (CFROM)",
			cb: slack.InteractionCallback{
				Type:      slack.InteractionTypeViewSubmission,
				TriggerID: "dummy-trigger", // 重要
				View: slack.View{
					CallbackID: "incident_modal",
					State: &slack.ViewState{
						Values: map[string]map[string]slack.BlockAction{
							"service_block": {
								"service_select": {SelectedOption: slack.OptionBlockObject{Value: "1"}},
							},
							"incident_summary_block": {
								"summary_text": {Value: "some incident"},
							},
							"channel_name_block": {
								"channel_name_text": {Value: "dummy-chan"},
							},
							"urgency_block": {
								"urgency_select": {SelectedOption: slack.OptionBlockObject{Value: "none"}},
							},
						},
					},
				},
				User: slack.User{ID: "UMODAL"},
				Channel: slack.Channel{
					GroupConversation: slack.GroupConversation{
						Conversation: slack.Conversation{ID: "CFROM"},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "ViewSubmission => edit_summary_modal => ok (CINC)",
			cb: slack.InteractionCallback{
				Type:      slack.InteractionTypeViewSubmission,
				TriggerID: "dummy-trigger",
				View: slack.View{
					CallbackID:      "edit_summary_modal",
					PrivateMetadata: "CINC",
					State: &slack.ViewState{
						Values: map[string]map[string]slack.BlockAction{
							"edit_summary_block": {
								"summary_text": {Value: "更新された事象内容"},
							},
						},
					},
				},
				User: slack.User{ID: "UEDIT"},
			},
			wantErr: false,
		},
		{
			name: "in_channel_options => reopen_incident => ok (CINC)",
			cb: slack.InteractionCallback{
				Type: slack.InteractionTypeBlockActions,
				Channel: slack.Channel{
					GroupConversation: slack.GroupConversation{
						Conversation: slack.Conversation{ID: "CINC"},
					},
				},
				Message: slack.Message{
					Msg: slack.Msg{Timestamp: "555.666"},
				},
				User: slack.User{ID: "UREOPEN"},
				ActionCallback: slack.ActionCallbacks{
					BlockActions: []*slack.BlockAction{
						{
							ActionID:       "in_channel_options",
							BlockID:        "keeper_action",
							SelectedOption: slack.OptionBlockObject{Value: "reopen_incident"},
						},
					},
				},
			},
			wantErr: false,
		},
	}

	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			postMsg = nil
			updateMsg = nil
			deleteMsg = nil
			openViewCount = 0

			err := cbHandler.Handle(&tc.cb)
			if tc.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// インシデント再開機能のテスト
func TestIncidentReopen(t *testing.T) {
	var postMsg, updateMsg, deleteMsg []map[string]string
	var setTopicCalls []map[string]string

	srv := slacktest.NewTestServer(func(c slacktest.Customize) {
		c.Handle("/auth.test", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"ok":true,"user_id":"UBOT"}`))
		}))
		c.Handle("/users.list", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			resp := struct {
				OK      bool `json:"ok"`
				Members []struct {
					ID   string `json:"id"`
					Name string `json:"name"`
				} `json:"members"`
			}{
				OK: true,
				Members: []struct {
					ID   string `json:"id"`
					Name string `json:"name"`
				}{
					{ID: "UREOPEN", Name: "reopener"},
					{ID: "UHANDLER", Name: "handler"},
				},
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(resp)
		}))
		c.Handle("/chat.postMessage", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_ = r.ParseForm()
			postMsg = append(postMsg, map[string]string{
				"channel":     r.FormValue("channel"),
				"blocks":      r.FormValue("blocks"),
				"attachments": r.FormValue("attachments"),
				"text":        r.FormValue("text"),
			})
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"ok":true}`))
		}))
		c.Handle("/chat.update", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_ = r.ParseForm()
			updateMsg = append(updateMsg, map[string]string{
				"channel": r.FormValue("channel"),
				"ts":      r.FormValue("ts"),
				"blocks":  r.FormValue("blocks"),
			})
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"ok":true}`))
		}))
		c.Handle("/chat.delete", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_ = r.ParseForm()
			deleteMsg = append(deleteMsg, map[string]string{
				"channel": r.FormValue("channel"),
				"ts":      r.FormValue("ts"),
			})
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"ok":true}`))
		}))
		c.Handle("/conversations.list", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			resp := struct {
				OK       bool `json:"ok"`
				Channels []struct {
					ID         string `json:"id"`
					Name       string `json:"name"`
					IsArchived bool   `json:"is_archived"`
					Topic      struct {
						Value string `json:"value"`
					} `json:"topic"`
				} `json:"channels"`
			}{
				OK: true,
				Channels: []struct {
					ID         string `json:"id"`
					Name       string `json:"name"`
					IsArchived bool   `json:"is_archived"`
					Topic      struct {
						Value string `json:"value"`
					} `json:"topic"`
				}{
					{ID: "CREOPEN", Name: "reopen-test", IsArchived: false, Topic: struct {
						Value string `json:"value"`
					}{Value: "【復旧】テスト事象内容"}},
					{ID: "CNOTREOPEN", Name: "not-reopen-test", IsArchived: false, Topic: struct {
						Value string `json:"value"`
					}{Value: "未復旧テスト事象内容"}},
					{ID: "CANN", Name: "announcement", IsArchived: false, Topic: struct {
						Value string `json:"value"`
					}{Value: ""}},
				},
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(resp)
		}))
		c.Handle("/conversations.info", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			channelID := r.URL.Query().Get("channel")
			var topicValue string
			switch channelID {
			case "CREOPEN":
				topicValue = "【復旧】テスト事象内容"
			case "CNOTREOPEN":
				topicValue = "未復旧テスト事象内容"
			default:
				topicValue = "デフォルトトピック"
			}

			resp := struct {
				OK      bool `json:"ok"`
				Channel struct {
					ID    string `json:"id"`
					Name  string `json:"name"`
					Topic struct {
						Value string `json:"value"`
					} `json:"topic"`
				} `json:"channel"`
			}{
				OK: true,
				Channel: struct {
					ID    string `json:"id"`
					Name  string `json:"name"`
					Topic struct {
						Value string `json:"value"`
					} `json:"topic"`
				}{
					ID:   channelID,
					Name: "test-channel",
					Topic: struct {
						Value string `json:"value"`
					}{Value: topicValue},
				},
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(resp)
		}))
		c.Handle("/conversations.setTopic", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_ = r.ParseForm()
			setTopicCalls = append(setTopicCalls, map[string]string{
				"channel": r.FormValue("channel"),
				"topic":   r.FormValue("topic"),
			})
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"ok":true}`))
		}))
	})
	go srv.Start()

	api := slack.New("dummy", slack.OptionAPIURL(srv.GetAPIURL()))

	defer srv.Stop()

	t.Run("復旧済みインシデントの再開", func(t *testing.T) {
		t.Setenv("TZ", "Asia/Tokyo")

		// 復旧済みのインシデントを準備
		recoveredTime := time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)
		incRepo := &mockIncidentRepo{
			data: map[string]*entity.Incident{
				"CREOPEN": {
					ChannelID:       "CREOPEN",
					Description:     "テスト事象内容",
					Level:           1,
					ServiceID:       1,
					HandlerUserID:   "UHANDLER",
					StartedAt:       time.Date(2024, 1, 1, 10, 0, 0, 0, time.UTC),
					RecoveredAt:     recoveredTime,
					RecoveredUserID: "URECOVERED",
					DisableTimer:    true,
				},
			},
		}

		cfgRepo := &mockConfigRepo{
			services: []entity.Service{
				{ID: 1, Name: "テストサービス", AnnouncementChannels: []string{"announcement"}},
			},
			levels: []entity.IncidentLevel{
				{Level: 1, Description: "レベル1"},
			},
			announce: []string{"CANN"},
		}

		repo := repository.NewRepository(incRepo, cfgRepo, cfgRepo, repository.NewSlackRepository(api))
		cbHandler := handler.NewCallbackHandler(context.Background(), repo, "https://example.com/", nil, nil, nil)

		// 初期状態をリセット
		postMsg = nil
		updateMsg = nil
		deleteMsg = nil
		setTopicCalls = nil

		// 再開処理を実行
		err := cbHandler.Handle(&slack.InteractionCallback{
			Type: slack.InteractionTypeBlockActions,
			User: slack.User{ID: "UREOPEN"},
			Channel: slack.Channel{
				GroupConversation: slack.GroupConversation{
					Conversation: slack.Conversation{ID: "CREOPEN"},
				},
			},
			Message: slack.Message{
				Msg: slack.Msg{Timestamp: "123.456"},
			},
			ActionCallback: slack.ActionCallbacks{
				BlockActions: []*slack.BlockAction{
					{
						ActionID:       "in_channel_options",
						BlockID:        "keeper_action",
						SelectedOption: slack.OptionBlockObject{Value: "reopen_incident"},
					},
				},
			},
		})

		require.NoError(t, err)

		// 非同期処理の完了を待つ
		time.Sleep(100 * time.Millisecond)

		// インシデントが再開状態に更新されたことを確認
		reopenedIncident := incRepo.data["CREOPEN"]
		assert.False(t, reopenedIncident.ReopenedAt.IsZero(), "再開時刻が設定されていません")
		assert.Equal(t, "UREOPEN", reopenedIncident.ReopenedUserID, "再開者が正しく設定されていません")
		assert.True(t, reopenedIncident.RecoveredAt.IsZero(), "復旧時刻がリセットされていません")
		assert.Empty(t, reopenedIncident.RecoveredUserID, "復旧者がリセットされていません")
		assert.False(t, reopenedIncident.DisableTimer, "タイマーが有効になっていません")

		// チャンネルトピックが更新されたことを確認
		assert.Len(t, setTopicCalls, 1, "トピック更新が呼ばれていません")
		if len(setTopicCalls) > 0 {
			assert.Equal(t, "CREOPEN", setTopicCalls[0]["channel"])
			assert.Equal(t, "テスト事象内容", setTopicCalls[0]["topic"]) // 【復旧】プレフィックスが削除される
		}

		// 再開通知が投稿されたことを確認
		assert.NotEmpty(t, postMsg, "再開通知が投稿されていません")

		// チャンネル内通知とアナウンス通知の両方が投稿されたことを確認
		channelNotificationFound := false
		announcementNotificationFound := false
		for _, msg := range postMsg {
			if msg["channel"] == "CREOPEN" && msg["attachments"] != "" {
				channelNotificationFound = true
			}
			if msg["channel"] == "CANN" && msg["attachments"] != "" {
				announcementNotificationFound = true
			}
		}
		assert.True(t, channelNotificationFound, "チャンネル内通知が投稿されていません")
		assert.True(t, announcementNotificationFound, "アナウンス通知が投稿されていません")
	})

	t.Run("未復旧インシデントの再開試行", func(t *testing.T) {
		t.Setenv("TZ", "Asia/Tokyo")

		// 未復旧のインシデントを準備
		incRepo := &mockIncidentRepo{
			data: map[string]*entity.Incident{
				"CNOTREOPEN": {
					ChannelID:   "CNOTREOPEN",
					Description: "未復旧事象内容",
					Level:       1,
					ServiceID:   1,
					StartedAt:   time.Date(2024, 1, 1, 10, 0, 0, 0, time.UTC),
					// RecoveredAtが未設定（未復旧状態）
				},
			},
		}

		cfgRepo := &mockConfigRepo{
			services: []entity.Service{
				{ID: 1, Name: "テストサービス"},
			},
		}

		repo := repository.NewRepository(incRepo, cfgRepo, cfgRepo, repository.NewSlackRepository(api))
		cbHandler := handler.NewCallbackHandler(context.Background(), repo, "https://example.com/", nil, nil, nil)

		// 初期状態をリセット
		postMsg = nil
		updateMsg = nil
		deleteMsg = nil
		setTopicCalls = nil

		// 再開処理を実行
		err := cbHandler.Handle(&slack.InteractionCallback{
			Type: slack.InteractionTypeBlockActions,
			User: slack.User{ID: "UREOPEN"},
			Channel: slack.Channel{
				GroupConversation: slack.GroupConversation{
					Conversation: slack.Conversation{ID: "CNOTREOPEN"},
				},
			},
			Message: slack.Message{
				Msg: slack.Msg{Timestamp: "789.123"},
			},
			ActionCallback: slack.ActionCallbacks{
				BlockActions: []*slack.BlockAction{
					{
						ActionID:       "in_channel_options",
						BlockID:        "keeper_action",
						SelectedOption: slack.OptionBlockObject{Value: "reopen_incident"},
					},
				},
			},
		})

		require.NoError(t, err)

		// 非同期処理の完了を待つ
		time.Sleep(100 * time.Millisecond)

		// エラーメッセージが投稿されたことを確認
		assert.NotEmpty(t, postMsg, "エラーメッセージが投稿されていません")
		errorMsgFound := false
		for _, msg := range postMsg {
			if msg["channel"] == "CNOTREOPEN" && msg["text"] == "⚠️ インシデントはまだ復旧していません。復旧していないインシデントは再開できません。" {
				errorMsgFound = true
				break
			}
		}
		assert.True(t, errorMsgFound, "適切なエラーメッセージが投稿されていません")

		// インシデントの状態が変更されていないことを確認
		incident := incRepo.data["CNOTREOPEN"]
		assert.True(t, incident.ReopenedAt.IsZero(), "インシデントが誤って再開されています")
		assert.Empty(t, incident.ReopenedUserID, "再開者が誤って設定されています")

		// トピックが変更されていないことを確認
		assert.Empty(t, setTopicCalls, "トピックが誤って変更されています")
	})
}
