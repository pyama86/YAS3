package handler_test

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/slack-go/slack"
	"github.com/slack-go/slack/slacktest"
	"github.com/stretchr/testify/assert"

	"github.com/pyama86/YAS3/domain/entity"
	"github.com/pyama86/YAS3/domain/repository"
	"github.com/pyama86/YAS3/handler"
)

// 全体周知モーダルの送信で、global_announcement_channels のみに周知されることを検証する
func TestGlobalAnnounceSubmit(t *testing.T) {
	var postMsg []map[string]string
	srv := slacktest.NewTestServer(func(c slacktest.Customize) {
		c.Handle("/chat.postMessage", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_ = r.ParseForm()
			postMsg = append(postMsg, map[string]string{
				"channel":     r.FormValue("channel"),
				"attachments": r.FormValue("attachments"),
				"text":        r.FormValue("text"),
			})
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"ok":true}`))
		}))
		c.Handle("/conversations.list", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			resp := map[string]any{
				"ok": true,
				"channels": []map[string]any{
					{"id": "CANN", "name": "ann", "is_archived": false},
					{"id": "CINC", "name": "inc", "is_archived": false},
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
		"CINC": {
			ChannelID:   "CINC",
			ServiceID:   1,
			Urgency:     "error",
			Description: "事象内容",
			StartedAt:   time.Date(2026, 6, 9, 14, 30, 0, 0, time.UTC),
		},
	}}
	cfgRepo := &mockConfigRepo{services: []entity.Service{{ID: 1, Name: "svc"}}}
	repo := repository.NewRepository(incRepo, cfgRepo, cfgRepo, repository.NewSlackRepository(api))
	// global_announcement_channels に "ann" を設定（手動周知の対象）
	config := &repository.Config{
		GlobalAnnouncementChannels: []string{"ann"},
		NotificationType:           "none",
	}
	cbHandler := handler.NewCallbackHandler(context.Background(), repo, "https://example.com/", nil, nil, config)

	cb := slack.InteractionCallback{
		Type: slack.InteractionTypeViewSubmission,
		View: slack.View{
			CallbackID:      "global_announce_modal",
			PrivateMetadata: "CINC",
			State: &slack.ViewState{
				Values: map[string]map[string]slack.BlockAction{
					"summary_block":  {"summary_text": {Value: "発生事象です"}},
					"occurred_block": {"occurred_text": {Value: "2026-06-09 14:30"}},
					"impact_block":   {"impact_text": {Value: "影響あり"}},
				},
			},
		},
		User: slack.User{ID: "U1"},
	}

	err := cbHandler.Handle(&cb)
	assert.NoError(t, err)

	postedToGlobal := false
	for _, p := range postMsg {
		if p["channel"] == "CANN" && p["attachments"] != "" {
			postedToGlobal = true
		}
	}
	assert.True(t, postedToGlobal, "global チャンネル(CANN)に周知が投稿されるべき: %+v", postMsg)
}
