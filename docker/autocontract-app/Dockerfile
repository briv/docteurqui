FROM debian:10-slim

# Useful for now to update the Doctor contact data.
VOLUME /docker-vols/doctor-data
# Used to access key material
# VOLUME /docker-vols/secrets
# Used to access key material
# VOLUME /docker-vols/mailinglist

ARG this_user=autcontract-app-user
# RUN groupadd --gid 2000 "$this_user" \
    # && useradd --uid 2000 --gid 2000 "$this_user"
RUN useradd -ms /bin/bash "$this_user"
WORKDIR /home/"$this_user"

# Our executable.
COPY ./autocontract "bin/autocontract"

COPY ./www "data/www"
COPY ./contract-template.html "data/contract-template.html"

RUN chown -R "$this_user" bin/ \
    && chown -R "$this_user" data/

USER "$this_user"
EXPOSE 18080
CMD ./bin/autocontract \
    -p=18080 \
    -http-data=./data/www \
    -dr-data-file=/docker-vols/doctor-data/data.txt \
    -pdf-template-file=./data/contract-template.html \
    -pdf-browser-devtools-url="$PDF_GEN_URL" \
    -pdf-internal-web-hostname="$PDF_INTERNAL_WEB_HOSTNAME" \
    -censor-key="$SECRET_CENSOR_KEY" \
    -mailinglist-file=/docker-vols/mailinglist/mailinglist \
    -mailinglist-pubkey-file=/docker-vols/secrets/mailinglist.public.key
