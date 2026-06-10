package odds_test

import (
	"testing"

	"github.com/team/websocket-poc/internal/models"
	"github.com/team/websocket-poc/internal/odds"
)

func TestInitialOdds(t *testing.T) {
	e := odds.NewEngine()
	updates := e.ProcessEvent("match1", &models.Event{Type: "match_started", MatchTime: 0}, &models.SportEventStatus{HomeScore: 0, AwayScore: 0})

	if len(updates) != 2 {
		t.Fatalf("expected 2 updates, got %d", len(updates))
	}
	if updates[0].Market != "1x2" {
		t.Errorf("expected 1x2 market, got %s", updates[0].Market)
	}
	if updates[1].Market != "over_under_2.5" {
		t.Errorf("expected over_under_2.5 market, got %s", updates[1].Market)
	}
	// Initial odds should be roughly balanced
	for _, o := range updates[0].Outcomes {
		if o.Odds < 1.0 {
			t.Errorf("odds should be >= 1.0, got %f for %s", o.Odds, o.Name)
		}
	}
}

func TestGoalShiftsOdds(t *testing.T) {
	e := odds.NewEngine()
	// Initial state
	before := e.ProcessEvent("match1", &models.Event{Type: "match_started", MatchTime: 0}, &models.SportEventStatus{HomeScore: 0, AwayScore: 0})
	// Home scores
	after := e.ProcessEvent("match1", &models.Event{Type: "score_change", MatchTime: 30, Competitor: "home"}, &models.SportEventStatus{HomeScore: 1, AwayScore: 0})

	homeOddsBefore := findOdds(before[0].Outcomes, "home")
	homeOddsAfter := findOdds(after[0].Outcomes, "home")

	// Home odds should decrease (team more likely to win)
	if homeOddsAfter >= homeOddsBefore {
		t.Errorf("home odds should decrease after scoring: before=%f after=%f", homeOddsBefore, homeOddsAfter)
	}
}

func TestRedCardShiftsOdds(t *testing.T) {
	e := odds.NewEngine()
	before := e.ProcessEvent("match1", &models.Event{Type: "match_started", MatchTime: 0}, &models.SportEventStatus{HomeScore: 0, AwayScore: 0})
	// Away gets red card
	after := e.ProcessEvent("match1", &models.Event{Type: "red_card", MatchTime: 50, Competitor: "away"}, &models.SportEventStatus{HomeScore: 0, AwayScore: 0})

	homeOddsBefore := findOdds(before[0].Outcomes, "home")
	homeOddsAfter := findOdds(after[0].Outcomes, "home")

	// Home odds should decrease (opponent weakened)
	if homeOddsAfter >= homeOddsBefore {
		t.Errorf("home odds should decrease after away red card: before=%f after=%f", homeOddsBefore, homeOddsAfter)
	}
}

func TestOverUnderShiftsOnGoal(t *testing.T) {
	e := odds.NewEngine()
	before := e.ProcessEvent("match1", &models.Event{Type: "match_started", MatchTime: 0}, &models.SportEventStatus{HomeScore: 0, AwayScore: 0})
	// Two goals scored early
	e.ProcessEvent("match1", &models.Event{Type: "score_change", MatchTime: 10, Competitor: "home"}, &models.SportEventStatus{HomeScore: 1, AwayScore: 0})
	after := e.ProcessEvent("match1", &models.Event{Type: "score_change", MatchTime: 20, Competitor: "away"}, &models.SportEventStatus{HomeScore: 1, AwayScore: 1})

	overBefore := findOdds(before[1].Outcomes, "over_2.5")
	overAfter := findOdds(after[1].Outcomes, "over_2.5")

	// Over odds should decrease (more likely to go over with 2 goals in 20 min)
	if overAfter >= overBefore {
		t.Errorf("over odds should decrease after goals: before=%f after=%f", overBefore, overAfter)
	}
}

func findOdds(outcomes []odds.Outcome, name string) float64 {
	for _, o := range outcomes {
		if o.Name == name {
			return o.Odds
		}
	}
	return 0
}
