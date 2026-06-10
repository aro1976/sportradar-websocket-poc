package odds

import (
	"math"
	"sync"
	"time"

	"github.com/team/websocket-poc/internal/models"
)

type Outcome struct {
	Name string  `json:"name"`
	Odds float64 `json:"odds"`
}

type OddsUpdate struct {
	MatchID   string    `json:"match_id"`
	Market    string    `json:"market"`
	Outcomes  []Outcome `json:"outcomes"`
	Timestamp time.Time `json:"timestamp"`
}

type matchState struct {
	homeScore int
	awayScore int
	minute    int
	redHome   int
	redAway   int
}

type Engine struct {
	mu     sync.Mutex
	states map[string]*matchState
}

func NewEngine() *Engine {
	return &Engine{states: make(map[string]*matchState)}
}

func (e *Engine) ProcessEvent(matchID string, event *models.Event, status *models.SportEventStatus) []OddsUpdate {
	e.mu.Lock()
	defer e.mu.Unlock()

	st, ok := e.states[matchID]
	if !ok {
		st = &matchState{}
		e.states[matchID] = st
	}

	if status != nil {
		st.homeScore = status.HomeScore
		st.awayScore = status.AwayScore
	}
	if event != nil {
		st.minute = event.MatchTime
		if event.Type == "red_card" {
			if event.Competitor == "home" {
				st.redHome++
			} else {
				st.redAway++
			}
		}
	}

	now := time.Now()
	return []OddsUpdate{
		{MatchID: matchID, Market: "1x2", Outcomes: calc1X2(st), Timestamp: now},
		{MatchID: matchID, Market: "over_under_2.5", Outcomes: calcOverUnder(st), Timestamp: now},
	}
}

func calc1X2(st *matchState) []Outcome {
	// Base probabilities adjusted by score, time, and red cards
	homeStr := 0.4
	drawStr := 0.3
	awayStr := 0.3

	// Score impact
	diff := float64(st.homeScore - st.awayScore)
	homeStr += diff * 0.15
	awayStr -= diff * 0.15

	// Red card impact
	homeStr += float64(st.redAway-st.redHome) * 0.05
	awayStr += float64(st.redHome-st.redAway) * 0.05

	// Time decay: as match progresses, current state matters more
	timeFactor := float64(st.minute) / 90.0
	if diff > 0 {
		homeStr += timeFactor * 0.1
		drawStr -= timeFactor * 0.05
		awayStr -= timeFactor * 0.05
	} else if diff < 0 {
		awayStr += timeFactor * 0.1
		drawStr -= timeFactor * 0.05
		homeStr -= timeFactor * 0.05
	}

	// Normalize
	homeStr = clamp(homeStr, 0.05, 0.9)
	drawStr = clamp(drawStr, 0.05, 0.9)
	awayStr = clamp(awayStr, 0.05, 0.9)
	total := homeStr + drawStr + awayStr

	return []Outcome{
		{Name: "home", Odds: round(total / homeStr)},
		{Name: "draw", Odds: round(total / drawStr)},
		{Name: "away", Odds: round(total / awayStr)},
	}
}

func calcOverUnder(st *matchState) []Outcome {
	totalGoals := float64(st.homeScore + st.awayScore)
	remaining := math.Max(0, float64(90-st.minute)) / 90.0
	expectedMore := remaining * 2.5

	overProb := 0.5
	if totalGoals+expectedMore > 2.5 {
		overProb = 0.5 + (totalGoals+expectedMore-2.5)*0.15
	} else {
		overProb = 0.5 - (2.5-totalGoals-expectedMore)*0.15
	}
	overProb = clamp(overProb, 0.05, 0.95)

	return []Outcome{
		{Name: "over_2.5", Odds: round(1.0 / overProb)},
		{Name: "under_2.5", Odds: round(1.0 / (1.0 - overProb))},
	}
}

func clamp(v, min, max float64) float64 {
	if v < min {
		return min
	}
	if v > max {
		return max
	}
	return v
}

func round(v float64) float64 {
	return math.Round(v*100) / 100
}
