import http from 'k6/http';
import { check } from 'k6';
import { options } from './options.js';

export { options };

export default function () {
  const url = `${__ENV.TARGET}/api/v1/workspaces/${__ENV.WORKSPACE_SLUG}/documents/${__ENV.DOCUMENT_ID}/signed-url`;
  const res = http.post(url, null, {
    headers: { Authorization: `Bearer ${__ENV.API_TOKEN}` },
  });
  check(res, {
    'status is 200 or 201': (r) => [200, 201].includes(r.status),
    'response time < 500ms': (r) => r.timings.duration < 500,
  });
}
