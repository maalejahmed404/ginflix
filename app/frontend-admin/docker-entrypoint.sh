#!/bin/sh

# Substitute env vars in template
envsubst '${BACKEND_URL} ${STREAM_URL} ${KEYCLOAK_CLIENT_ID} ${KEYCLOAK_REALM} ${KEYCLOAK_HOST}' < /usr/share/nginx/html/index.html.template > /usr/share/nginx/html/index.html

# Run NGINX
exec "$@"
