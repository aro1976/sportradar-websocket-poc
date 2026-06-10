# Skill: Sportradar API Integration Guide

## Overview

This guide covers how to migrate from Sportradar Simulations (free, no auth) to Production APIs (paid, requires API key). It details all available Soccer APIs, authentication, and code changes needed.

## Current State (PoC)

```go
// No auth, uses playback.sportradar.com
client.StreamEvents(ctx, recordingID, handler)
```

## Production State (Target)

```go
// Requires x-api-key header, uses api.sportradar.com
client.StreamEvents(ctx, sportEventID, handler)
```

---

## Authentication

All production Sportradar APIs require an API key passed as a header:

```
x-api-key: {your_api_key}
```

### How to obtain:
1. Go to https://marketplace.sportradar.com
2. Subscribe to the desired product (Soccer API, Odds API, etc.)
3. API key will be available in your account dashboard
4. Trial: 30 days free, self-provisioned (except Push feeds which require sales contact)

---

## Available Soccer APIs

### 1. Soccer Push Events (Real-Time Streaming)

**What it does**: Streams live match events (goals, cards, shots, substitutions) as they happen.

**Access level**: Requires **Realtime** plan (contact sales)

**Endpoint**:
```
GET https://api.sportradar.com/soccer/{access_level}/v4/stream/events/subscribe
Header: x-api-key: {key}
```

**Filter by match**:
```
GET https://api.sportradar.com/soccer/production/v4/stream/events/subscribe?format=json&sport_event_id=sr:sport_event:13468929
```

**Response format**: HTTP chunked streaming (same as Simulations), JSON payloads with heartbeat every 5s.

**Code change needed**:
```go
// internal/sportradar/stream.go
// Change URL from:
//   playback.sportradar.com/subscribe/events?recording_id={id}
// To:
//   api.sportradar.com/soccer/production/v4/stream/events/subscribe?sport_event_id={id}
// Add header:
//   x-api-key: {key}
```

---

### 2. Soccer Push Statistics (Real-Time Streaming)

**What it does**: Streams live team/player statistics updates.

**Endpoint**:
```
GET https://api.sportradar.com/soccer/{access_level}/v4/stream/statistics/subscribe
Header: x-api-key: {key}
```

---

### 3. Odds Comparison - Live Odds (REST, Polling)

**What it does**: Returns current live odds from multiple bookmakers. Updates every 15-30 seconds.

**Endpoint**:
```
GET https://api.sportradar.com/oddscomparison-live/production/v2/{language}/sport_events/{sport_event_id}/markets.json
Header: x-api-key: {key}
```

**Important**: This is REST (polling), NOT streaming. You need to poll every 15-30s.

**Code change needed**:
```go
// New file: internal/sportradar/odds_api.go
// Implement polling loop that fetches odds every 15-30s
// Parse response and publish to Redis same as the odds engine does today
```

---

### 4. Odds Comparison - Prematch (REST)

**What it does**: Pre-match odds from bookmakers before the game starts.

**Endpoint**:
```
GET https://api.sportradar.com/oddscomparison-prematch/production/v2/{language}/sport_events/{sport_event_id}/markets.json
Header: x-api-key: {key}
```

---

### 5. Soccer Probabilities (REST)

**What it does**: Win probabilities (home/draw/away) calculated by Sportradar models.

**Endpoint**:
```
GET https://api.sportradar.com/soccer-probabilities/production/v4/{language}/sport_events/{sport_event_id}/sport_event_probabilities.json
Header: x-api-key: {key}
```

---

### 6. Live Summaries (REST)

**What it does**: Summary of all currently live matches with scores, status, key stats.

**Endpoint**:
```
GET https://api.sportradar.com/soccer/production/v4/{language}/schedules/live/sport_event_summaries.json
Header: x-api-key: {key}
```

---

### 7. Live Timelines (REST)

**What it does**: Play-by-play timeline for currently live matches.

**Endpoint**:
```
GET https://api.sportradar.com/soccer/production/v4/{language}/schedules/live/timelines.json
Header: x-api-key: {key}
```

---

### 8. Sport Event Timeline (REST)

**What it does**: Full timeline for a specific match (past or live).

**Endpoint**:
```
GET https://api.sportradar.com/soccer/production/v4/{language}/sport_events/{sport_event_id}/timeline.json
Header: x-api-key: {key}
```

---

## Code Changes Required for Production

### 1. Add API Key configuration

```go
// internal/sportradar/client.go
type Client struct {
    http   *http.Client
    apiKey string
    env    string // "simulation" or "production"
}

func NewClient(apiKey string) *Client {
    env := "simulation"
    if apiKey != "" {
        env = "production"
    }
    return &Client{http: &http.Client{}, apiKey: apiKey, env: env}
}
```

### 2. Update StreamEvents for production

```go
// internal/sportradar/stream.go
func (c *Client) StreamEvents(ctx context.Context, sportEventID string, handler func(models.PushMessage)) error {
    var url string
    if c.env == "production" {
        url = fmt.Sprintf("https://api.sportradar.com/soccer/production/v4/stream/events/subscribe?format=json&sport_event_id=%s", sportEventID)
    } else {
        url = fmt.Sprintf("https://playback.sportradar.com/subscribe/events?recording_id=%s", sportEventID)
    }

    req, _ := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
    if c.apiKey != "" {
        req.Header.Set("x-api-key", c.apiKey)
    }
    // ... rest same as today
}
```

### 3. Add Odds API poller (new, for real odds)

```go
// internal/sportradar/odds_poller.go
func (c *Client) PollLiveOdds(ctx context.Context, sportEventID string, interval time.Duration, handler func(OddsResponse)) error {
    url := fmt.Sprintf("https://api.sportradar.com/oddscomparison-live/production/v2/en/sport_events/%s/markets.json", sportEventID)
    ticker := time.NewTicker(interval)
    defer ticker.Stop()

    for {
        select {
        case <-ctx.Done():
            return ctx.Err()
        case <-ticker.C:
            req, _ := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
            req.Header.Set("x-api-key", c.apiKey)
            resp, err := c.http.Do(req)
            if err != nil { continue }
            var odds OddsResponse
            json.NewDecoder(resp.Body).Decode(&odds)
            resp.Body.Close()
            handler(odds)
        }
    }
}
```

### 4. Environment variables

```bash
# Production
SPORTRADAR_API_KEY=your_key_here
SPORTRADAR_ENV=production
SPORT_EVENT_ID=sr:sport_event:13468929

# Simulation (current, no key needed)
SPORTRADAR_ENV=simulation
RECORDING_ID=6e62c450-2d31-11f1-a030-116f91a2eb34
```

### 5. Switch odds source

In production, you have two choices:

| Approach | Source | Latency | Cost |
|----------|--------|---------|------|
| **A. Real odds (Odds API)** | Poll bookmaker odds every 15-30s | 15-30s | Odds API subscription |
| **B. Generated odds (current)** | Generate from Push Events | <1s | Soccer Push subscription only |
| **C. Hybrid** | Push Events for fast updates + Odds API for calibration | <1s | Both subscriptions |

**Recommendation**: Start with **B** (generated odds from Push Events) since it's already working and gives sub-second latency. Add Odds API later for calibration/validation.

---

## Migration Checklist

- [ ] Obtain Sportradar API key (trial or paid)
- [ ] Request Realtime plan access for Push feeds (contact sales)
- [ ] Add `SPORTRADAR_API_KEY` env var to ECS task definitions
- [ ] Update `internal/sportradar/client.go` with dual-mode (simulation/production)
- [ ] Update `internal/sportradar/stream.go` URL and auth header
- [ ] Test with trial key against live matches
- [ ] (Optional) Implement Odds API poller for real bookmaker odds
- [ ] Update CDK to pass API key as ECS secret (Secrets Manager)
- [ ] Update health check to verify Sportradar connection status
- [ ] Monitor API rate limits (check response headers for remaining quota)

---

## Rate Limits

| API | Typical Limit |
|-----|--------------|
| Push Events | No limit (one connection per subscription) |
| Push Statistics | No limit (one connection per subscription) |
| REST endpoints | ~1000 requests/minute (varies by plan) |
| Odds API | ~100-500 requests/minute |

Rate limit headers in response:
```
X-RateLimit-Limit: 1000
X-RateLimit-Remaining: 999
X-RateLimit-Reset: 1234567890
```

---

## Differences: Simulation vs Production

| Aspect | Simulation | Production |
|--------|-----------|------------|
| URL | playback.sportradar.com | api.sportradar.com |
| Auth | None | x-api-key header |
| Data | Replayed recordings | Live real matches |
| Availability | Always (on-demand replay) | Only during live matches |
| Cost | Free | Paid subscription |
| Push format | Identical JSON structure | Identical JSON structure |
| Heartbeat | Every 5s | Every 5s |

**Key point**: The JSON payload structure is IDENTICAL between simulation and production. No parser changes needed.
