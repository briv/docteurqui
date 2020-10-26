# docteurqui.com

# Commands to get going

- Launch front-end:
```sh
cd src/web/contract-page
yarn dev
```

- Launch go backend:
```sh
cd src/backend
go run cmd/autocontract/main.go \
  -dev \
  -http-proxy 1234 \
  -dr-data-file ../../tmp/PS_LibreAcces_Personne_activite_202005090902.txt \
  -pdf-template-file ../contract-templates/dist/index.html \
  -pdf-internal-web-hostname host.docker.internal

# Can also add the following
  -mailinglist-file="$HOME/Desktop/mailinglist"
```

- Launch chrome back-end for PDF generation
```sh
docker run -it --rm \
  --cap-add=SYS_ADMIN \
  -p 9222:9222 \
   --entrypoint "/bin/sh" \
   autocontract/pdf-gen
# then the chromium process to launch within:
chromium-browser --headless --disable-gpu --remote-debugging-address=0.0.0.0 --remote-debugging-port=9222
```

- Read the mailing list
```sh
./src/backend/cmd/dev-mailinglist/cat_remote_mailinglist.sh
```

# TODOs
- fix secrets ending up within the systemd Unit file

- check how the form interacts with CORS and Origin/Referrer headers
- sitemap.xml ? see christine.website source code
- canonicalization of contract periods (plus would help with counting number of unique contracts)
- checkout "github.com/justinas/alice" for easier golang http.Handler chaining
- investigate requestAnimationFrame (and 'fastdom' lib) for batching DOM measurements/mutations, especially when submitting form and clearing / adding errors.
- add date of rempla to generated PDF (i.e. "Contrat remplacement - 2 Avril 2020.pdf")
- use https://github.com/go-sourcemap/sourcemap to print nicer error messages by using JS sourcemaps.
- fix autocomplete menu on desktop Safari (involves changing label/input "name" and "id" attrs so Safari doesn't guess about autocomplete)
- only allow numbers in RPPS fields
- add native sharing of the website on iOS with https://developer.mozilla.org/en-US/docs/Web/API/Navigator/share
- use an icon (see Stripe register page) next to the form error messages.
- use https://github.com/Polymer/lit-html for HTML generation within JS code ?
- fix error popping up on address field when user autocompletes with a Dr who has a blank "address" field.
- analytics/errors going though grafana + loki https://github.com/grafana/loki/tree/master/docs, with 25GB of disk space should be plenty before dedicated aggregated metrics are needed
- get current time for contract establishment from user's browser.
- use golangCI-lint for Go linting

- contract page numbers in headers and footers:
    - https://stackoverflow.com/questions/44575628/alter-the-default-header-footer-when-printing-to-pdf
    - https://stackoverflow.com/questions/46077392/additional-options-in-chrome-headless-print-to-pdf
    - https://stackoverflow.com/questions/51438984/puppeteer-header-and-footer-not-displayed-on-first-page
    - Use Prometheus for monitoring instance and general statistics about errors etc...
- Use nix to build docker images (reproducible + easier to make slim hopefully)
