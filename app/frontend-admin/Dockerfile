FROM nginx:alpine

# Install envsubst for variable substitution
RUN apk add --no-cache gettext

COPY index.html.template /usr/share/nginx/html/index.html.template
COPY silent-check-sso.html /usr/share/nginx/html/silent-check-sso.html

# Copy entrypoint
COPY docker-entrypoint.sh /docker-entrypoint.sh
RUN chmod +x /docker-entrypoint.sh

ENTRYPOINT ["/docker-entrypoint.sh"]
CMD ["nginx", "-g", "daemon off;"]
