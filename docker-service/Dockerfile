FROM golang:1.11-alpine

# Installs latest Chromium package.
RUN apk update && apk upgrade \
    && echo @edge http://nl.alpinelinux.org/alpine/edge/community >> /etc/apk/repositories \
    && echo @edge http://nl.alpinelinux.org/alpine/edge/main >> /etc/apk/repositories \
    && apk add --no-cache \
    freetype@edge \
    harfbuzz@edge \
    chromium@edge \
    nss@edge \
    fontconfig@edge

RUN rm -rf \
    /var/lib/apt/lists/* \
    /var/cache/apk/* \
    /usr/share/man \
    /tmp/*

# Add Chrome as a user
RUN mkdir -p /usr/src/app \
    && adduser -D chrome \
    && chown -R chrome:chrome /usr/src/app

# TMP: add commands below this line for testing
RUN mkdir -p /home/chrome/.fonts
ADD ./fonts/* /home/chrome/.fonts/

# Run Chrome as non-privileged
USER chrome

RUN fc-cache -f -v

WORKDIR /usr/src/app

# ENV CHROME_BIN=/usr/bin/chromium-browser \
#     CHROME_PATH=/usr/lib/chromium/

# Autorun chrome headless with no GPU, plus remote debugging set up
ENTRYPOINT ["chromium-browser", "--headless", "--disable-gpu", "--remote-debugging-address=0.0.0.0", "--remote-debugging-port=9222"]
