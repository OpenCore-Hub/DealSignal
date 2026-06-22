import http from 'k6/http';
import { check } from 'k6';
import { options } from './options.js';

export { options };

export default function () {
  const url = `${__ENV.TARGET}/api/v1/workspaces/${__ENV.WORKSPACE_SLUG}/search?q=${__ENV.QUERY || 'revenue'}&limit=10`;
  const res = http.get(url, {
    headers: { Authorization: `Bearer ${__ENV.API_TOKEN}` },
  });
  check(res, {
    'status is 200': (r) => r.status === 200,
    'response time < 1000ms': (r) => r.timings.duration < 1000,
  });
}
