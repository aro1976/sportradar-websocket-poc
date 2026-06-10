# PoC: WebSocket Fan-Out for Real-Time Betting Odds

## Overview
Proof of Concept to validate architecture of WebSocket + fan-out with real Soccer data from Sportradar, targeting 100k+ concurrent connections with sub-second latency.

## Decisions Log

### 1. Scope: Proof of Concept (not MVP or full system)
- **Chosen**: PoC — validate WebSocket + fan-out architecture with real Sportradar data
- **Alternatives considered**:
  - a. MVP completo — user registers, deposits, bets, withdraws (too broad for validation)
  - b. Core de apostas — betting engine + WebSocket + odds without payments (still too much for validation)
- **Rationale**: Need to validate that the architecture handles 100k concurrent connections with sub-second latency before investing in business logic

### 2. Stack: Go
- **Chosen**: Go
- **Alternatives considered**:
  - a. Node.js/TypeScript — good for I/O, large ecosystem, but single-threaded; needs workers for 100k connections
  - b. Java/Kotlin (Spring WebFlux) — mature, reactive, good enterprise ecosystem; more verbose, higher memory
  - c. Elixir/Phoenix — excellent for massive WebSocket (millions of connections), fault-tolerant; smaller developer pool
  - d. Go — excellent concurrency (goroutines), low memory (~10KB/connection), native performance
- **Rationale**: For 100k concurrent WebSocket connections with low latency, Go's goroutine model is ideal. Each connection costs ~10KB memory, and goroutines handle I/O multiplexing natively. Team is open to Go.

### 3. Infrastructure: Local Docker Compose + Load Testing
- **Chosen**: Local Docker Compose with k6 load testing simulating 100k connections
- **Alternatives considered**:
  - a. Local only (Docker Compose) — validate architecture before spending on infra
  - b. Direct AWS deployment — ECS/Redis in cloud for real load testing
  - c. Local + load test simulating AWS scale — local infra with k6/artillery simulating 100k connections
- **Rationale**: Option C gives the best of both worlds — no cloud cost during development, but still validates performance at scale. Can move to AWS after PoC proves the architecture works.

### 4. Target Cloud: AWS
- **Chosen**: AWS (for eventual production deployment)
- **Architecture designed for**:
  - ECS Fargate for WebSocket servers
  - ElastiCache Redis for Pub/Sub
  - NLB (Network Load Balancer) for WebSocket connections
  - NOT ALB (idle timeout limit of 4000s, adds L7 parsing latency, throttles at ~100k connections)
  - NOT API Gateway WebSocket (too expensive at 100k+ connections with high message frequency)

### 4.1 Load Balancer: NLB over ALB
- **Chosen**: NLB (Network Load Balancer, Layer 4)
- **Alternatives considered**:
  - a. ALB (Layer 7) — native WebSocket support, path routing, but has idle timeout limit (4000s), adds ~1-2ms latency from HTTP parsing, potential throttling at 100k+ connections
  - b. NLB (Layer 4) — TCP passthrough, no idle timeout (connections live indefinitely), handles millions of concurrent connections, ~100μs added latency
  - c. API Gateway WebSocket — fully managed but $12k+/month at our message volume
- **Rationale**: Betting WebSocket connections last 90+ minutes (a full match). NLB has no idle timeout, scales to millions of connections, and adds minimal latency (TCP passthrough). ALB's 4000s idle timeout would require client-side reconnection logic. NLB is purpose-built for long-lived TCP connections at scale. Health checks and metrics endpoints are exposed on a separate internal ALB or accessed directly via service discovery.

### 5. Data Source: Sportradar Simulations (free) + Simulated Odds
- **Chosen**: Sportradar Simulations for game events + locally generated odds
- **Alternatives considered**:
  - a. Sportradar Odds API real (polling every 15-30s) — requires paid access, polling not push
  - b. Sportradar Simulations (free) + simulated odds from events — no cost, no key needed, validates fan-out
  - c. Both — simulations for development, real API when key available
- **Rationale**: The Odds API is polling-based (15-30s refresh), not push/streaming. The Soccer Push API requires a paid "Realtime" plan. Simulations are free, require no API key, and replay real games — perfect for PoC. We generate odds locally based on game events (goal → odds shift, red card → odds shift, etc.)

### 6. Real-Time Delivery: WebSocket (own servers, not AWS API Gateway)
- **Chosen**: Self-managed WebSocket servers on ECS/EKS
- **Alternatives considered**:
  - a. AWS API Gateway WebSocket — managed, but charges per message and has 500 conn/sec ramp-up limit
  - b. Self-managed WebSocket servers — full control, cheaper at scale, better latency
- **Rationale**: At 100k+ connections with high-frequency odds updates, API Gateway becomes prohibitively expensive and adds unnecessary latency. Self-managed Go servers handle 30-50k connections per instance.

### 7. Fan-Out Mechanism: Redis Pub/Sub
- **Chosen**: Redis Pub/Sub via ElastiCache
- **Alternatives considered**:
  - a. Redis Pub/Sub — sub-millisecond latency within VPC, simple, proven
  - b. Amazon SNS → SQS — more durable but adds latency (tens of ms)
  - c. Apache Kafka — durable, replayable, but overkill for ephemeral odds updates
  - d. NATS — lightweight, fast, but less ecosystem support on AWS
- **Rationale**: Odds are ephemeral (only latest matters), so durability isn't needed. Redis Pub/Sub gives <1ms fan-out latency within a VPC and is trivial to operate via ElastiCache.

### 8. Latency Requirement: Sub-second
- **Chosen**: <1 second end-to-end (Sportradar event → client receives updated odds)
- **Breakdown target**:
  - Sportradar → Ingest: ~50-200ms (network)
  - Ingest → Redis: <1ms
  - Redis → WebSocket Server: <1ms
  - WebSocket Server → Client: <50ms (local), <200ms (internet)
  - Total: <500ms typical

### 9. User Type: B2C
- **Chosen**: B2C — end users bet directly
- **Implication for PoC**: The WebSocket protocol must be browser-compatible (standard WebSocket, not gRPC streaming or custom protocols)

### 10. Betting Markets: Broad (eventual)
- **Chosen**: All major markets — 1X2, over/under, both teams score, handicap, correct score, goalscorer, corners, cards
- **Implication for PoC**: Odds engine will simulate a subset (1X2, over/under) to prove the concept; structure supports expansion

### 11. Coverage: Pre-match + Live (eventual)
- **Chosen**: Both pre-match and live betting
- **Implication for PoC**: Focus on live events (Simulations replays live matches), pre-match is simpler and doesn't stress the architecture

### 12. Odds Source: External (Sportradar Odds API — eventual)
- **Chosen**: Consume odds from Sportradar Odds API (for production)
- **PoC approach**: Generate simulated odds locally based on game events since Odds API requires paid access

## Architecture

```
┌─────────────────────┐
│ Sportradar          │
│ Simulations API     │
│ (HTTP Chunked)      │
└─────────┬───────────┘
          │ Events stream (goals, cards, etc.)
          ▼
┌─────────────────────┐
│ Ingest Service      │
│ (Go)                │
│ + Odds Engine       │
└─────────┬───────────┘
          │ Publish odds JSON
          ▼
┌─────────────────────┐
│ Redis Pub/Sub       │
│ Channel: odds:{id}  │
└──┬──────┬──────┬────┘
   │      │      │
   ▼      ▼      ▼
┌─────┐┌─────┐┌─────┐
│ WS1 ││ WS2 ││ WSN │  WebSocket Servers (Go)
└──┬──┘└──┬──┘└──┬──┘
   │      │      │
   ▼      ▼      ▼
┌─────────────────────┐
│  NLB (TCP:443)      │  No idle timeout, millions of connections
│  TLS termination    │
└─────────┬───────────┘
          │
          ▼
      100k+ Clients (browser WebSocket)
```

### AWS Production Topology
```
Clients → Route 53 → NLB (TCP:443, TLS) → ECS Fargate WebSocket Servers (:8080)
                                                    ↕
                                              ElastiCache Redis Pub/Sub
                                                    ↕
                                              ECS Fargate Ingest ← Sportradar API

Internal: ALB (internal) → /metrics, /health (Prometheus scraping, health checks)
```

## Sportradar API Details

### Simulations (used in PoC)
- **Recordings list**: POST `https://playback.sportradar.com/graphql` with query `recordingsBySport(sport: "soccer")`
- **Push stream**: GET `https://playback.sportradar.com/subscribe/events?recording_id={recordingId}`
- **REST (with session)**: GET `https://playback.sportradar.com/replay/soccer/{recordingId}?feed=summary&contentType=json&sessionId={sessionId}`
- **No authentication required**
- Available feeds for Soccer: events (push), statistics (push), summary (rest), timeline (rest), lineups (rest)

### Production APIs (future)
- **Soccer Push Events**: `https://api.sportradar.com/soccer/{access_level}/v4/stream/events/subscribe` (requires Realtime plan)
- **Odds Comparison Live Odds**: REST polling every 15-30s (requires subscription)
- **Authentication**: `x-api-key` header

## Task Breakdown

### Task 1: Project Setup + Sportradar Simulations Connection
- **Objective**: Create Go project, connect to Sportradar Simulations, consume Soccer events
- **Implementation**:
  - `go mod init github.com/team/websocket-poc`
  - HTTP client consuming chunked stream from Simulations Push API
  - GraphQL client to list available Soccer recordings
  - JSON parser for event payloads (goal, card, shot, etc.)
  - Structured logging of received events
- **Tests**: Unit test for event parser; integration test connecting to Simulations
- **Demo**: Run service, see live Soccer events streaming in terminal

### Task 2: Odds Engine
- **Objective**: Transform game events into simulated odds changes
- **Implementation**:
  - Data model: OddsUpdate{MatchID, Market, Outcomes[], Timestamp}
  - Markets: 1X2 (home/draw/away), Over/Under 2.5
  - Event handlers: goal → big shift, red card → moderate shift, time → drift
  - Interface: `OddsEngine.ProcessEvent(event) → []OddsUpdate`
- **Tests**: Unit tests for each event type producing correct odds movement
- **Demo**: Pipe Simulations events into engine, see odds updating per event

### Task 3: Redis Pub/Sub Integration
- **Objective**: Add Redis as fan-out layer between ingest and WebSocket servers
- **Implementation**:
  - Docker Compose with Redis
  - Publisher: Ingest publishes OddsUpdate JSON to `odds:{match_id}` channel
  - Subscriber: test consumer printing messages from channel
  - Message format: `{"match_id":"...","market":"1x2","outcomes":[...],"timestamp":"..."}`
- **Tests**: Integration test: publish → subscribe → validate message
- **Demo**: `docker-compose up`, see odds flowing through Redis channels

### Task 4: WebSocket Server + Client Broadcast
- **Objective**: WebSocket server accepting browser connections, broadcasting odds
- **Implementation**:
  - HTTP server with WebSocket upgrade (nhooyr.io/websocket or gorilla/websocket)
  - Hub pattern: connection registry, subscribe to match channels
  - Client sends `{"subscribe":"match_id"}` → server subscribes Redis channel
  - Server broadcasts odds updates to all clients on that match
  - Ping/pong heartbeat, dead connection cleanup
- **Tests**: Integration test: connect WS client, subscribe, verify odds received
- **Demo**: Multiple browser tabs connected, all receiving live odds from same match

### Task 5: Docker Compose Full Stack
- **Objective**: Orchestrate all services locally with multiple WebSocket server replicas
- **Implementation**:
  - Multi-stage Dockerfile for Go service
  - docker-compose.yml: Redis, Ingest, WebSocket (3 replicas), Nginx
  - Nginx: upstream block, WebSocket upgrade support, sticky sessions (ip_hash)
  - Environment variables for config (REDIS_URL, RECORDING_ID, etc.)
- **Tests**: `docker-compose up` starts cleanly; health check endpoints
- **Demo**: Full system running locally, connect via Nginx, receive odds from replicated WebSocket servers

### Task 6: Load Test with k6
- **Objective**: Validate 100k concurrent WebSocket connections with sub-second latency
- **Implementation**:
  - k6 script with xk6-websocket extension
  - Ramp scenario: 0 → 1k → 10k → 50k → 100k over 2-5 minutes
  - Metrics: connection time, message latency (p50/p95/p99), errors, throughput
  - Summary report at end
- **Tests**: Run at 1k, 10k, 50k, 100k; validate p99 latency < 1s at each tier
- **Demo**: Execute load test showing 100k active connections with latency metrics

### Task 7: Observability (Prometheus + Grafana)
- **Objective**: Real-time visibility into system performance during load tests
- **Implementation**:
  - Prometheus metrics in WebSocket server: `ws_connections_active`, `ws_messages_sent_total`, `ws_fanout_latency_seconds`
  - Prometheus + Grafana in Docker Compose
  - Grafana dashboard: connections per server, fan-out latency histogram, memory/CPU
- **Tests**: `/metrics` endpoint returns valid Prometheus format
- **Demo**: Run load test with Grafana dashboard showing real-time performance

## Success Criteria
- [ ] 100k concurrent WebSocket connections maintained
- [ ] Odds updates delivered in < 1 second (p99) from event to client
- [ ] System remains stable during 5+ minute sustained load
- [ ] No message loss under normal operation
- [ ] Linear horizontal scaling (2x servers → 2x capacity)
