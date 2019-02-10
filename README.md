#Commands to get going

Launch docker container:
    docker run -it --rm --cap-add=SYS_ADMIN -p 9222:9222 -v /tmp:/usr/src/app --entrypoint "/bin/sh" testing
then the chromium process to launch within:
    chromium-browser --headless --disable-gpu --remote-debugging-address=0.0.0.0 --remote-debugging-port=9222

- Launch front-end:
```bash
cd src/web
yarn dev
```

- Launch go backend:
```bash
cd src/backend
go run autocontract.go -d 1234
```

# TODOs
- must haves, improve UI/UX:
    - ERROR management ! show user something if it goes wrong (7 to do it seems)
    - :hover styles on buttons !!
    - save any temporary filled data in case the person leaves the page before submitting the form.
    - change button styles to make primary "submit" button pop out WAY more
        see https://lospec.com/palette-list/
    - check https://www.smashingmagazine.com/2018/08/best-practices-for-mobile-form-design/ for tips on form design.
    - have a footer/header with info about the site and links to other sites
    - make desktop version better
    - analytics for ME:
        - try grafana + loki https://github.com/grafana/loki/tree/master/docs, with 25GB of disk space should be plenty before dedicated aggregated metrics are needed
        - from back-end, log:
            - a line whith subRRPS | regular RPPS | timestamp on each contract that is genereated
            - a line whenever an error occurs (whether in JS or in backend), with some details: timestamp | requestUUID | errorMessage
- nice to haves
    - use a real icon instead of emoji X for "delete" buttons
    - simplify app.js

- use golangCI-lint for Go linting

# archive TODOs:
- reorganize to put everything together:
    - yarn stuff for website + contracts
    - golang "app" server that serves website as well
    - docker with chromium
- headers and footers:
    - https://stackoverflow.com/questions/44575628/alter-the-default-header-footer-when-printing-to-pdf
    - https://stackoverflow.com/questions/46077392/additional-options-in-chrome-headless-print-to-pdf
    - https://stackoverflow.com/questions/51438984/puppeteer-header-and-footer-not-displayed-on-first-page
- Docker with Chromium + Golang for piloting the PDF creation
- add fonts to docker linux for good pdf generation (Lucida Grande for example)
- Golang template for HTML
- Use Prometheus for monitoring instance and general statistics about errors etc...
- Use nix to build docker images (reproducible + easier to make slim hopefully)

- use https://github.com/szimek/signature_pad to gather JS signature on website for PDF contract

# links
https://yann.hodique.info/blog/using-nix-to-build-docker-images/

https://ariya.io/2016/06/isolated-development-environment-using-nix
https://stackoverflow.com/questions/50242387/how-to-record-a-reproducible-profile-in-nix-especially-from-nix-env
https://nixos.wiki/wiki/Development_environment_with_nix-shell


https://stackoverflow.com/questions/2793751/how-can-i-create-download-link-in-html/16316830#16316830
