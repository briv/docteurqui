# Commands to get going

- Launch front-end:
```sh
cd src/js/web
yarn dev
```

- Launch go backend:
```sh
cd src/backend
go run autocontract.go \
  -dev \
  -http-data ../web/dist \
  -dr-data-file ../../tmp/PS_LibreAcces_Personne_activite_202003041023.txt \
  -pdf-template-file ../contract-templates/dist/index.html \
  -pdf-internal-web-hostname host.docker.internal
```

- Launch chrome back-end for PDF generation
```sh
docker run -it --rm --cap-add=SYS_ADMIN -p 9222:9222 -v /tmp:/usr/src/app --entrypoint "/bin/sh" testing
```
then the chromium process to launch within:
```sh
chromium-browser --headless --disable-gpu --remote-debugging-address=0.0.0.0 --remote-debugging-port=9222
```

# TODOs
- fix secrets ending up within the systemd Unit file

- check how the form interacts with CORS and Origin/Referrer headers
- hide doctor search suggestions when we get a 422 because of query length (> 40 characters)
- sitemap.xml ? see christine.website source code
- canonicalization of contract periods (plus would help with counting number of unique contracts)
- checkout "github.com/justinas/alice" for easier golang http.Handler chaining
- investigate requestAnimationFrame (and 'fastdom' lib) for batching DOM measurements/mutations, especially when submitting form and clearing / adding errors.
- allow keyboard use to select autocomplete entries
- add date of rempla to generated PDF (i.e. "Contrat remplacement - 2 Avril 2020.pdf")
- use https://github.com/go-sourcemap/sourcemap to print nicer error messages by using JS sourcemaps.
- fix autocomplete menu on desktop Safari (involves changing label/input "name" and "id" attrs so Safari doesn't guess about autocomplete)
- only allow numbers in RPPS fields
- add native sharing of the website on iOS with https://developer.mozilla.org/en-US/docs/Web/API/Navigator/share
- use a real icon instead of emoji X for "delete" buttons
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
