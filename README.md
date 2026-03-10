# Ghep event pusher

Ghep er en Github App som pusher Github events for teams til Slack.
Det som skiller Ghep fra en haug av andre lignende tjenester er at den automagisk henter repoer basert på Github _teamet_ ditt, og pusher forskjellige events til forskjellige kanaler!

## Hvordan ser det ut?

### Commits

Commits er kanskje den mest aktive hendelsen for et team.
For hver `push` blir det sendt en hendelse som blir postet til Slack.
Ghep vil prøve å lenke til `Co-Authors` så godt som mulig.

Hvis en `push` trigger en workflow så vil Ghep reacte på commits basert på reisen til workflowen.

👀 - når en jobb har blitt satt i kø  
⏳ - når den kjører  
✅ - fullført vellykket  
❌ - fullført feilet  
🅿️ - fullført kansellert  

![Commits posted to Slack](images/commits.png)

### Issues og pull requests

Issues og pull requests blir behandlet nesten likt, og ser like ut når de havner i Slack.
`merged` og `deleted` hendelser vil bli posted i Slack-tråden til et issue eller pull requests.
Dette gjør det enkelt for dere å følge med på hva som skjer.

![A issue posted to Slack](images/issue.png)

![A pull request posted to Slack](images/pull-request.png)

### Workflows

Workflows som er vellykket er ikke så interessant, derfor er det kun workflows som feiler som blir postet til Slack.

![A failed workflow will be posted to Slack](images/failed-workflow.png)

### Releases

Sender releases til en egen kanal, `draft`, `prerelease`, og `releases` blir sendt ut.

![A release posted to Slack](images/release.png)

### Security

Sender code scanning, secret scanning, Dependabot, og security advisory til egen kanal.
Noen av disse hendelsene kan man filtrere på alvorlighetsgrad.

![A secret scanning alert posted to Slack](images/secret-scanning.png)

## Ta den i bruk

Alt du trenger å gjøre er å redigere [`.nais/teams.yaml`](https://github.com/navikt/ghep/blob/main/.nais/teams.yaml) og legge til ditt team og deres kanaler.

``` yaml
nada:
 commits: "#nada-commits"
 issues: "#nada-issues"
 pulls: "#nada-pull-requests"
 workflows: "#nada-ci"
 releases: "#nada-releases"
 security: "#nada-security"
```

PS: Hvis kanalene dine er private må du selv invitere @ghep inn i hver kanal.

### Konfigurering

Vi har også støtte for litt konfigurering.
Dette legges under `teamnavn.config`.

#### Team configuration

``` yaml
team:
  config:
    ignoreRepositories:
      - repoA
      - repoB
    silenceDependabot: "always"
    externalContributorsChannel: "#channel"
    security:
      severityFilter: "high"
    pingSlackUsers: true
```

- `ignoreRepositories` - En liste med repositories man ikke ønsker hendelser fra
- `silenceDependabot` - Hvis denne blir satt til `always` så ignorer man alle hendelser fra Dependabot
- `externalContributorsChannel` - Issues og pull requests fra brukere som ikke er i teamet ditt vil havne i en egen kanal
- `security.severityFilter` - Filtrer ut sikkerhetshendelser som har _lavere_ alvorlighetsgrad enn spesifisert
- `pingSlackUsers`- Pinger Slack-brukere som er tildelt issues eller pull requests

#### Pull Requests

``` yaml
team:
  pulls: #channel
  config:
    pulls:
      ignoreBots: bool
      ignoreDrafts: bool
      minimalist: bool
```

- `ignoreBots` - Ikke få Slack-melding om Pull Request opprettet av bots
- `ignoreDrafts` - Ignorer draft pull requests
- `minimalist` - Don't post the body as attachment, only add title to the message

#### Workflows

``` yaml
team:
  workflows: #channel
  config:
    workflows:
      ignoreBots: bool
      branches: [string]
      workflows: [string]
      repositories: [string]
```

- `ignoreBots` - Ikke få Slack-melding om workflows som feiler for bots (for eksempel Dependabot)
- `branches` - Få *kun* Slack-melding om workflows som feiler for spesifikke branches
- `workflows` - Få *kun* Slack-melding om workflows som feiler for spesifikke workflows
- `repositories` - Få *kun* Slack-melding om workflows som feiler for spesifikke repositories

## Lokal utvikling

Kjør opp Postgres for testing med Docker.

``` shell
docker run --name postgres -e POSTGRES_PASSWORD=postgres -p 5432:5432 -d postgres
docker run --name adminer --link postgres:db -p 8081:8080 -d adminer
psql -h localhost -U postgres -c 'CREATE DATABASE ghep;'
```

## Kontakt oss

Ta kontakt i `#ghep-værsågod` på Slack hvis du har noen spørsmål.
