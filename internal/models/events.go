package models

import "time"

type Recording struct {
	ID        string `json:"id"`
	Title     string `json:"title"`
	Sport     string `json:"sport"`
	League    string `json:"league"`
	Scheduled string `json:"scheduled"`
	APIs      []API  `json:"apis"`
}

type API struct {
	Name    string   `json:"name"`
	APIType string   `json:"apiType"`
	Formats []string `json:"formats"`
}

type PushMessage struct {
	Heartbeat *Heartbeat `json:"heartbeat,omitempty"`
	Payload   *Payload   `json:"payload,omitempty"`
	Metadata  *Metadata  `json:"metadata,omitempty"`
}

type Heartbeat struct {
	From     int64  `json:"from"`
	To       int64  `json:"to"`
	Interval int    `json:"interval"`
	Type     string `json:"type"`
	Package  string `json:"package"`
}

type Payload struct {
	SportEventStatus *SportEventStatus `json:"sport_event_status,omitempty"`
	Event            *Event            `json:"event,omitempty"`
}

type SportEventStatus struct {
	Status      string `json:"status"`
	MatchStatus string `json:"match_status"`
	HomeScore   int    `json:"home_score"`
	AwayScore   int    `json:"away_score"`
}

type Event struct {
	ID          int64     `json:"id"`
	Type        string    `json:"type"`
	Time        time.Time `json:"time"`
	MatchTime   int       `json:"match_time"`
	MatchClock  string    `json:"match_clock,omitempty"`
	Competitor  string    `json:"competitor"`
	Period      int       `json:"period"`
	PeriodType  string    `json:"period_type"`
	HomeScore   int       `json:"home_score,omitempty"`
	AwayScore   int       `json:"away_score,omitempty"`
	Players     []Player  `json:"players,omitempty"`
	Method      string    `json:"method,omitempty"`
	Outcome     string    `json:"outcome,omitempty"`
}

type Player struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Type string `json:"type,omitempty"`
}

type Metadata struct {
	Format        string `json:"format"`
	SportEventID  string `json:"sport_event_id"`
	EventID       string `json:"event_id"`
	Channel       string `json:"channel"`
	CompetitionID string `json:"competition_id"`
	SportID       string `json:"sport_id"`
	SeasonID      string `json:"season_id"`
}
