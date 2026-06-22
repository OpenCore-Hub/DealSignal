import http from 'k6/http';
import { check } from 'k6';
import { options } from './options.js';

export { options };

export default function () {
  const res = http.get(`${__ENV.TARGET}/api/v1/public/links/${__ENV.LINK_TOKEN}`);
  check(res, {
    'status is 200 or 302': (r) => [200, 302].includes(r.status),
    'response time < 500ms': (r) => r.timings.duration < 500,
  });
}
