#!/bin/sh
set -e

API_HOST="${API_HOST:-api}"
export API_HOST

envsubst '\$API_HOST' < /etc/nginx/conf.d/default.conf.template > /etc/nginx/conf.d/default.conf

exec nginx -g 'daemon off;'
