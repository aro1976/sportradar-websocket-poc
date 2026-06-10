import ws from 'k6/ws';
import { check, sleep } from 'k6';
import { Counter, Trend } from 'k6/metrics';

const wsUrl = __ENV.WS_URL || 'ws://WebSoc-NLB55-FBqc5kImJlPf-7f13f5caa3b8794f.elb.sa-east-1.amazonaws.com/ws';
const matchId = __ENV.MATCH_ID || 'sr:sport_event_id:67644872';

const messagesReceived = new Counter('ws_messages_received');
const messageLatency = new Trend('ws_message_latency_ms');

export const options = {
  stages: [
    { duration: '30s', target: 1000 },
    { duration: '30s', target: 10000 },
    { duration: '1m', target: 50000 },
    { duration: '1m', target: 100000 },
    { duration: '2m', target: 100000 }, // sustained
    { duration: '30s', target: 0 },
  ],
  thresholds: {
    ws_message_latency_ms: ['p(99)<1000'],
  },
};

export default function () {
  const res = ws.connect(wsUrl, {}, function (socket) {
    socket.on('open', () => {
      socket.send(JSON.stringify({ subscribe: matchId }));
    });

    socket.on('message', (msg) => {
      messagesReceived.add(1);
      try {
        const data = JSON.parse(msg);
        if (data.timestamp) {
          const sent = new Date(data.timestamp).getTime();
          const latency = Date.now() - sent;
          if (latency > 0 && latency < 30000) {
            messageLatency.add(latency);
          }
        }
      } catch (e) {}
    });

    socket.on('error', () => {});

    // Keep connection open for the duration of the VU iteration
    sleep(10);
    socket.close();
  });

  check(res, { 'connected successfully': (r) => r && r.status === 101 });
}
