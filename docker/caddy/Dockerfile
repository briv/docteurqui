FROM caddy:2.0.0-rc.3


ARG this_user=caddyuser
# RUN useradd -ms /bin/bash "$this_user"
RUN mkdir -p /home/"$this_user"
WORKDIR /home/"$this_user"

COPY ./Caddyfile Caddyfile
COPY ./www-root "www-root"

# RUN chown -R "$this_user" bin/ \
#     && chown -R "$this_user" data/

# USER "$this_user"

CMD AUTOCONTRACT_HOSTNAME="$AUTOCONTRACT_HOSTNAME" \
    caddy run \
    --config ./Caddyfile \
    --adapter caddyfile
