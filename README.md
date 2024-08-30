# Ghep event pusher

Ghep er en Github App som pusher Github events for teams til Slack.
Det som skiller Ghep fra en haug av andre lignende tjenester er at den automagisk henter repoer basert på Github teamet ditt, og pusher forskjellige events til forskjellige kanaler!

## Ta den i bruk

Alt du trenger å gjøre er å redigere `.nais/teams.yaml` og legge til ditt team og deres kanaler.

``` yaml
nada:
 commits: "#nada-commits"
 issues: "#nada-issues"
 pulls: "#nada-pull-requests"
 workflows: "#nada-ci"
```

### Konfigurering

Vi har også støtte for litt konfigurering.
Dette legges under `teamnavn.config` og hver sin type.

#### Issues and pull requests

Disse konfigurasjonene gjelder for både issues og pull requests.

``` yaml
team:
  config:
    externalContributorsChannel: "#channel"
```

- `externalContributorsChannel` - Bidrag fra brukere som ikke er i teamet vil havne i en egen kanal

#### Workflows

``` yaml
team:
  workflows: #channel
  config:
    workflows:
      ignoreBots: bool
      branches: [string]
```

- `ignoreBots` - Ikke få Slack-melding om workflows som feiler for bots (for eksempel Dependabot)
- `branches` - Få kun Slack-melding om workflows som feiler for spesifikke branches

## Lokal utvikling

Kjør opp Redis for testing med Docker.

``` shell
docker run --name redis -p 6379:6379 -d redis
```

## Kontakt oss

Ta kontakt i `#ghep-værsågod` på Slack hvis du har noen spørsmål.
