# WebSocket Fan-Out PoC — Real-Time Betting Odds

Proof of Concept validating WebSocket fan-out architecture for 100k+ concurrent connections with sub-second latency, using real Soccer data from Sportradar Simulations.

## Architecture

```
Sportradar Simulations (HTTP SSE) → Ingest + Odds Engine → Redis Pub/Sub → WebSocket Servers (x3) → Clients
                                                                                    ↑
                                                                              NLB (TCP:80)
```

### AWS Production Topology

```
Clients → Route 53 → NLB (TCP:80) → ECS Fargate WebSocket Servers (:8080)
                                              ↕
                                        ElastiCache Redis Pub/Sub
                                              ↕
                                        ECS Fargate Ingest ← Sportradar Simulations API
```

## Quick Start (Local)

```bash
# Start all services
docker-compose up --build

# Services:
# - WebSocket: ws://localhost:8080/ws
# - Prometheus: http://localhost:9090
# - Grafana: http://localhost:3000 (admin/admin)
```

## Quick Start (AWS)

```bash
cd infra
npm install
npx cdk deploy --profile clouddog-dev
```

**Live endpoint:** `ws://WebSoc-NLB55-FBqc5kImJlPf-7f13f5caa3b8794f.elb.sa-east-1.amazonaws.com/ws`

## Connect as a Client

```javascript
const ws = new WebSocket('ws://localhost:8080/ws');
ws.onopen = () => ws.send(JSON.stringify({subscribe: 'sr:sport_event_id:67644872'}));
ws.onmessage = (e) => console.log(JSON.parse(e.data));
```

Or open `index.html` in a browser for a visual dashboard.

## Run Load Test

```bash
# Install k6: brew install k6
k6 run loadtest/websocket.js

# Quick smoke test
k6 run --vus 10 --duration 10s loadtest/websocket.js

# Custom target
k6 run --stage "30s:1000,1m:10000,2m:10000,30s:0" loadtest/websocket.js
```

## Project Structure

```
cmd/ingest/          — Consumes Sportradar events, generates odds, publishes to Redis
cmd/wsserver/        — WebSocket server with Redis subscriber and Prometheus metrics
internal/models/     — Data models for Sportradar push events
internal/odds/       — Odds engine (event → odds calculation)
internal/pubsub/     — Redis Pub/Sub publisher/subscriber
internal/ws/         — WebSocket hub, client management, metrics
internal/sportradar/ — Sportradar Simulations API client
infra/               — AWS CDK infrastructure (ECS Fargate, NLB, ElastiCache)
loadtest/            — k6 load test scripts
grafana/             — Dashboard provisioning
```

## How It Works

1. **Ingest Service** connects to Sportradar Simulations API (free, no auth required) via HTTP SSE streaming
2. Receives real-time soccer events (goals, cards, shots, etc.)
3. **Odds Engine** transforms events into simulated odds (1X2, Over/Under 2.5)
4. Publishes odds to **Redis Pub/Sub** channel `odds:{match_id}`
5. **WebSocket Servers** subscribe to Redis channels and broadcast to connected clients
6. Clients subscribe to specific matches via `{"subscribe": "match_id"}`

## Sportradar Simulations API

No authentication required. Replays real soccer matches on demand.

```bash
# List available recordings
curl -X POST https://playback.sportradar.com/graphql \
  -H 'Content-Type: application/json' \
  -d '{"query":"query { recordingsBySport(sport: \"soccer\") { id title apis { name apiType } } }"}'

# Stream push events
curl -N https://playback.sportradar.com/subscribe/events?recording_id={recordingId}
```

## Odds Engine

Generates simulated odds based on match events:

| Event | Effect |
|-------|--------|
| Goal | Large shift in 1X2 odds toward scoring team |
| Red Card | Moderate shift against weakened team |
| Time passing | Gradual drift reinforcing current score |

Markets supported: **1X2** (home/draw/away), **Over/Under 2.5**

## AWS Infrastructure

Deployed to `sa-east-1` via CDK (`clouddog-dev` profile):

| Component | Spec | Purpose |
|-----------|------|---------|
| ECS Fargate (WS) | 1 vCPU, 2GB × 3 tasks | WebSocket servers |
| ECS Fargate (Ingest) | 0.25 vCPU, 512MB × 1 task | Event consumer + odds engine |
| ElastiCache Redis | cache.t4g.micro | Pub/Sub fan-out |
| NLB | TCP:80 → :8080, internet-facing | Load balancer (no idle timeout) |
| Auto-scaling | 3–10 tasks, CPU target 60% | Horizontal scaling |

### Why NLB over ALB?

- No idle timeout (WebSocket connections last 90+ min)
- Handles millions of concurrent TCP connections
- ~100μs added latency vs ~1-2ms for ALB
- Lower cost at scale

## Metrics & Observability

WebSocket server exposes Prometheus metrics at `/metrics`:

- `ws_connections_active` — Current active connections
- `ws_messages_sent_total` — Total messages broadcasted
- `ws_fanout_latency_seconds` — Time from Redis receive to client send

Grafana dashboard auto-provisioned with panels for connections, throughput, and latency distribution.

## Success Criteria

- [ ] 100k concurrent WebSocket connections maintained
- [ ] Odds updates delivered in < 1 second (p99)
- [ ] System stable during 5+ min sustained load
- [ ] No message loss under normal operation
- [ ] Linear horizontal scaling (2x servers → 2x capacity)

## Key Decisions

| Decision | Choice | Rationale |
|----------|--------|-----------|
| Language | Go | Goroutines handle 50k+ connections per instance, ~10KB/conn |
| Fan-out | Redis Pub/Sub | <1ms latency, ephemeral data (no durability needed) |
| Load Balancer | NLB | No idle timeout, millions of connections, TCP passthrough |
| Data Source | Sportradar Simulations | Free, no API key, replays real matches |
| Odds | Generated locally | Sportradar Odds API is polling-only (15-30s), not push |
| Not API Gateway WS | Self-managed | API GW would cost ~$12k/mo at 100k connections with high msg frequency |

See [IMPLEMENTATION_PLAN.md](IMPLEMENTATION_PLAN.md) for full decision log with alternatives considered.
