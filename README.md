docteurqui.com
--------------

[docteurqui.com](https://docteurqui.com) is a website for French family doctors
to create the necessary contract when they substitute (a French law
requirement).
You can try out the contract editor - what the users are interested in -
[here](https://docteurqui.com/outils/editeur-contrat-remplacement/).

### Architecture
The website is comprised of:
- a back-end in [Go](https://golang.org), which processes a form and turns it
into a PDF using Chromium. It also holds a search index to auto-complete
information for most French family doctors based on a fuzzy name search.
- an odd front-end which does not use any JavaScript frameworks, mostly to
experiment and keep total client payload small.

The whole package is then built using [Nix](https://nixos.org) and copied to a
server running NixOS. Browsers connect to a
[Caddy](https://caddyserver.com) reverse-proxy when accessing the site.

### Developing
Nix is used to provide the required dev environment:
```
$ nix-shell
```
[DEV.md](./DEV.md) has useful commands to launch front-end and back-ends once
in the nix shell.

### License
The code in this repository is (obviously) source-available, however it is *not*
open-source (see [LICENSE.txt](./LICENSE.txt)).
