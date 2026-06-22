import http from 'k6/http';
import { check } from 'k6';

export const options = {
  stages: [
    { duration: '30s', target: 5 },
    { duration: '1m', target: 20 },
    { duration: '30s', target: 0 },
  ],
  thresholds: {
    http_req_failed: ['rate<0.05'],
    http_req_duration: ['p(95)<3000', 'p(99)<5000'],
  },
};

export default function () {
  const url = `${__ENV.TARGET}/api/v1/workspaces/${__ENV.WORKSPACE_SLUG}/assistant/chat`;
  const payload = JSON.stringify({
    session_id: __ENV.SESSION_ID || '',
    message: __ENV.MESSAGE || 'Summarize the latest document',
  });
  const res = http.post(url, payload, {
    headers: {
      Authorization: `Bearer ${__ENV.API_TOKEN}`,
      'Content-Type': 'application/json',
    },
  });
  check(res, {
    'status is 200': (r) => r.status === 200,
    'response time < 5000ms': (r) => r.timings.duration < 5000,
  });
}
