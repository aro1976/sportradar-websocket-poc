package models_test

import (
	"testing"

	"github.com/team/websocket-poc/internal/models"
)

func TestParseHeartbeat(t *testing.T) {
	raw := `{"heartbeat":{"from":1724956680,"to":1724956685,"interval":5,"type":"events","package":"soccer-v4"}}`
	msg, err := models.ParsePushMessage([]byte(raw))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if msg.Heartbeat == nil {
		t.Fatal("expected heartbeat, got nil")
	}
	if msg.Heartbeat.Interval != 5 {
		t.Errorf("expected interval 5, got %d", msg.Heartbeat.Interval)
	}
	if msg.Heartbeat.Package != "soccer-v4" {
		t.Errorf("expected package soccer-v4, got %s", msg.Heartbeat.Package)
	}
}

func TestParseGoalEvent(t *testing.T) {
	raw := `{
		"payload":{
			"sport_event_status":{"status":"live","match_status":"1st_half","home_score":0,"away_score":1},
			"event":{"id":1830309249,"type":"score_change","match_time":15,"competitor":"away","period":1,"period_type":"regular_period","home_score":0,"away_score":1,"method":"header","players":[{"id":"sr:player:138601","name":"Uryga, Alan","type":"scorer"}]}
		},
		"metadata":{"format":"json","sport_event_id":"sr:sport_event_id:52633623","event_id":"score_change","channel":"soccer","competition_id":"sr:competition:34480","sport_id":"sr:sport:1","season_id":"sr:season:119783"}
	}`
	msg, err := models.ParsePushMessage([]byte(raw))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if msg.Payload == nil || msg.Payload.Event == nil {
		t.Fatal("expected payload with event")
	}
	e := msg.Payload.Event
	if e.Type != "score_change" {
		t.Errorf("expected score_change, got %s", e.Type)
	}
	if e.MatchTime != 15 {
		t.Errorf("expected match_time 15, got %d", e.MatchTime)
	}
	if e.Competitor != "away" {
		t.Errorf("expected competitor away, got %s", e.Competitor)
	}
	if len(e.Players) != 1 || e.Players[0].Name != "Uryga, Alan" {
		t.Errorf("unexpected players: %+v", e.Players)
	}
	if msg.Metadata.SportEventID != "sr:sport_event_id:52633623" {
		t.Errorf("unexpected sport_event_id: %s", msg.Metadata.SportEventID)
	}
}

func TestParseYellowCard(t *testing.T) {
	raw := `{
		"payload":{
			"sport_event_status":{"status":"live","match_status":"1st_half","home_score":0,"away_score":0},
			"event":{"id":1830321677,"type":"yellow_card","match_time":36,"competitor":"home","period":1,"period_type":"regular_period"}
		},
		"metadata":{"format":"json","sport_event_id":"sr:sport_event_id:52633607","event_id":"yellow_card","channel":"soccer"}
	}`
	msg, err := models.ParsePushMessage([]byte(raw))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if msg.Payload.Event.Type != "yellow_card" {
		t.Errorf("expected yellow_card, got %s", msg.Payload.Event.Type)
	}
}

func TestParseInvalidJSON(t *testing.T) {
	_, err := models.ParsePushMessage([]byte(`{invalid`))
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}
