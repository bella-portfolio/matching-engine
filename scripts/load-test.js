import http from 'k6/http';
import { check, sleep } from 'k6';
import { Counter, Trend } from 'k6/metrics';

// Custom metrics
const ordersPlaced = new Counter('orders_placed');
const orderLatency = new Trend('order_latency', true);

export const options = {
  // Target: 1000 TPS on 1 core / 2 GB
  scenarios: {
    constant_load: {
      executor: 'constant-arrival-rate',
      rate: 1000,          // 1000 iterations per second
      timeUnit: '1s',
      duration: '30s',     // Run for 30 seconds
      preAllocatedVUs: 50,
      maxVUs: 200,
    },
  },
  thresholds: {
    http_req_duration: ['p(95)<50'],    // 95% of requests < 50ms
    http_req_failed: ['rate<0.01'],     // < 1% failure rate
    'order_latency': ['p(95)<20'],      // 95% order processing < 20ms
  },
};

export default function () {
  const url = 'http://localhost:8080/orders';

  // Alternate buy/sell orders with varying prices
  const side = Math.random() > 0.5 ? 'BUY' : 'SELL';
  const price = side === 'BUY'
    ? (10000 + Math.random() * 100).toFixed(0)
    : (10000 + Math.random() * 100).toFixed(0);
  const quantity = (Math.random() * 10 + 1).toFixed(0);

  const payload = JSON.stringify({
    side: side,
    type: 'LIMIT',
    price: parseFloat(price),
    quantity: parseFloat(quantity),
  });

  const params = {
    headers: {
      'Content-Type': 'application/json',
    },
    timeout: '5s',
  };

  const res = http.post(url, payload, params);

  check(res, {
    'status is 200': (r) => r.status === 200,
    'response has order': (r) => JSON.parse(r.body).order !== undefined,
  });

  ordersPlaced.add(1);
  orderLatency.add(res.timings.duration);

  // No sleep — constant-arrival-rate handles pacing
}
