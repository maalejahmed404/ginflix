FROM nginx:alpine

# Install envsubst for variable substitution
RUN apk add --no-cache gettext

# Copy index.html
COPY docker-entrypoint.sh /docker-entrypoint.sh
COPY *.jpg /usr/share/nginx/html/
COPY *.mp3 /usr/share/nginx/html/
COPY index.html.template /usr/share/nginx/html/index.html.template

RUN chmod +x /docker-entrypoint.sh
# Ensure images are readable by nginx
RUN chmod 644 /usr/share/nginx/html/*.jpg
RUN chmod 644 /usr/share/nginx/html/*.mp3

ENTRYPOINT ["/docker-entrypoint.sh"]
CMD ["nginx", "-g", "daemon off;"]
