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
	evHandler := handler.NewEventHandler(context.Background(), api, incRepo)

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
				},
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(resp)
		}))
	})
	go srv.Start()
	defer srv.Stop()

	api := slack.New("dummy", slack.OptionAPIURL(srv.GetAPIURL()))
	incRepo := &mockIncidentRepo{data: map[string]*entity.Incident{
		"CINC":  {ChannelID: "CINC", ServiceID: 1, Urgency: "none"},
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
