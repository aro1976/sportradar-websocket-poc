# WebSocket Fan-Out PoC — Real-Time Betting Odds

Proof of Concept validating WebSocket fan-out architecture for 100k+ concurrent connections with sub-second latency, using real Soccer data from Sportradar Simulations.

## Quick Start

```bash
# Start all services
docker-compose up --build

# Services available:
# - WebSocket: ws://localhost:8080/ws
# - Prometheus: http://localhost:9090
# - Grafana: http://localhost:3000 (admin/admin)
```

## Connect as a client

```javascript
const ws = new WebSocket('ws://localhost:8080/ws');
ws.onopen = () => ws.send(JSON.stringify({subscribe: 'sr:sport_event_id:67644872'}));
ws.onmessage = (e) => console.log(JSON.parse(e.data));
```

## Run Load Test (k6)

```bash
# Install k6: brew install k6
k6 run loadtest/websocket.js
```

## Architecture

```
Sportradar Simulations → Ingest + Odds Engine → Redis Pub/Sub → WebSocket Servers (x3) → Clients
                                                                        ↑
                                                                  Nginx LB (port 8080)
```

## Project Structure

```
cmd/ingest/       — Consumes Sportradar events, generates odds, publishes to Redis
cmd/wsserver/     — WebSocket server with Redis subscriber and Prometheus metrics
internal/models/  — Data models for Sportradar push events
internal/odds/    — Odds engine (event → odds calculation)
internal/pubsub/  — Redis Pub/Sub publisher/subscriber
internal/ws/      — WebSocket hub, client management, metrics
internal/sportradar/ — Sportradar Simulations API client
loadtest/         — k6 load test scripts
grafana/          — Dashboard provisioning
```
