#!/bin/sh

# Substitute env vars in template
envsubst '${BACKEND_URL} ${STREAM_URL}' < /usr/share/nginx/html/index.html.template > /usr/share/nginx/html/index.html

# Run NGINX
exec "$@"
