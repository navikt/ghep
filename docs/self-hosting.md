# Required setup for self-hosting

Rough guide to required setup if you want to self-host the app.
This is not a step-by-step guide, but rather a list of things you need to do.

## Github App

Permissions are optional, but the app will only receive events for the permissions it has.
If you want to receive PR events, you need to grant the app read-only access to pull requests. 
The app will not be able to do anything with these permissions, but it needs them to receive the events.

| Permission             | Level     | Why                                                        |
|------------------------|-----------|------------------------------------------------------------|
| Metadata               | Read-only | Repository info, rename/public events, default branch info |
| Contents               | Read-only | Push/commit events                                         |
| Pull requests          | Read-only | PR events and digest query for open PRs                    |
| Issues                 | Read-only | Issue events                                               |
| Actions                | Read-only | workflow_run events                                        |
| Code scanning alerts   | Read-only | Security alert events                                      |
| Dependabot alerts      | Read-only | Dependabot alert events                                    |
| Secret scanning alerts | Read-only | Secret scanning alert events                               |

### Webhook

The Github app's job is to post subscribe to Github Events and post them using a webhook.
You need to set up a webhook endpoint that can receive these events and forward them to the Slack app.
The webhook endpoint will be `https://my-selfhosted-ghep.no/events`.
Make sure to configure a webhook secret in your GitHub App settings and set `GITHUB_WEBHOOK_SECRET`.
Ghep will reject any request with an invalid or missing `X-Hub-Signature-256` header.

In addition to the above Github permissions the configured webhook must check of relevant boxes in "Subscribe to events".
Note that not all of the events are supported by Ghep today.

#### Debugging webhooks

Attempted webhook deliveries can be viewed in the "Recent Deliveries" section of your Github App's settings.
This is useful for debugging and seeing the payloads of the events being sent to your webhook endpoint.

## Slack app

You will first need to create a Slack app and install it to your workspace.
Then you need to use the bot token (starting with `xoxb-`).
The bot token is used to authenticate the bot when it posts messages to Slack.

Use the [slack-app-manifest.json]() when creating a Slack app.

The Slack app will need the following permissions in order to work:

| Scope             | Why                                                         |
|-------------------|-------------------------------------------------------------|
| channels:join     | Join public channels in a workspace                         |
| channels:read     | View basic information about public channels in a workspace |
| chat:write        | Post and update messages                                    |
| chat:write.public | Post to channels the bot isn't a member of                  |
| reactions:read    | Read existing reactions before replacing them               |
| reactions:write   | Add/remove reactions                                        |
| users:read        | List workspace users                                        |
| users:read.email  | Read Slack profile emails for GitHub↔Slack user mapping     |

The Slack App will likely fail with error of missing scopes if they are not present:

## Env vars

In addition to the env vars you can read in [the nais yaml](../nais.yaml), you will also need to set the following env vars:

| Env var                    | Description                                                                                                                                                |
|----------------------------|------------------------------------------------------------------------------------------------------------------------------------------------------------|
| PGURL                      | The connection string to connect to your Postgres database                                                                                                 |
| GITHUB_APP_ID              | The ID of your Github app                                                                                                                                  |
| GITHUB_APP_INSTALLATION_ID | The installation ID of your Github app. You can find this in the URL when you go to your app's installation page. It is the number after `/installations/` |
| GITHUB_APP_PRIVATE_KEY     | The private key of your Github app, in PEM format.                                                                                                         |
| GITHUB_WEBHOOK_SECRET      | The webhook secret configured in your GitHub App settings.                                                                                                 |
| SLACK_TOKEN                | The bot token of your Slack app, starting with `xoxb-`                                                                                                     |


## Runtime environment

As this is not a 3rd party managed Slackbot, the container image will need to to run somewhere provided by you.
The ingress needs to be reachable from Github in order to receive Github event webhooks.

## Helm chart

We supply a [Helm chart](../chart) for those running in Kubernetes, published to `oci://ghcr.io/navikt/charts/ghep`.

### Prerequisites

1. A PostgreSQL database. Migrations run automatically on startup.
2. A Secret with database credentials and app secrets:

   ```sh
   kubectl create secret generic ghep-secrets \
     --from-literal=db-username=ghep \
     --from-literal=db-password=secret \
     --from-literal=slack-token=xoxb-... \
     --from-literal=github-app-id=123456 \
     --from-literal=github-app-installation-id=12345678 \
     --from-literal=github-webhook-secret=... \
     --from-file=github-app-private-key=private-key.pem
   ```

   Key names can be overridden via `existingSecret.keys` if your Secret uses different ones.

3. A ConfigMap with your teams configuration:

   ```sh
   kubectl create configmap ghep-teams --from-file=teams.yaml=teams.yaml
   ```

### Install

```sh
helm install ghep oci://ghcr.io/navikt/charts/ghep \
  --set githubOrg=my-org \
  --set existingSecret.name=ghep-secrets \
  --set database.host=my-postgres.example.com \
  --set database.name=ghep \
  --set teamsConfig.name=ghep-teams \
  --set ingress.enabled=true \
  --set ingress.hosts[0].host=ghep.example.com
```

See [values.yaml](../chart/values.yaml) for all options.

Note that the chart runs a single replica on purpose: Ghep performs leader election only on the Nais platform, and multiple replicas would send duplicate scheduled digests.
