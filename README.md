# hnykda/mattermost — bleve-restore fork

> **This is a personal fork of [mattermost/mattermost](https://github.com/mattermost/mattermost).**
> The `bleve-restore` branch restores the Bleve embedded search engine that was removed in v11.
> See below for details. Everything else is upstream.

---

## What this fork does

Mattermost v11 removed the [Bleve](https://github.com/blevesearch/bleve) pure-Go full-text search engine
that had worked well for self-hosted setups. The stated direction is Elasticsearch/OpenSearch, which is
overkill for a small personal instance and requires a paid license for the hosted offering.

Since Mattermost is AGPL, we restored the Bleve code from v10.5.0. The `SearchEngineInterface` contract
is unchanged between v10 and v11, so the implementation slots in cleanly as a single additive commit.

### Changes over upstream v11.5.x

| File | Change |
|------|--------|
| `server/platform/services/searchengine/bleveengine/` | Restored from v10.5.0 (index management, search, autocomplete) |
| `server/platform/services/searchengine/bleveengine/indexer/` | Background bulk indexer worker |
| `server/platform/services/searchengine/searchengine.go` | Added `BleveEngine` field to `Broker`, wired into `UpdateConfig`/`GetActiveEngines` |
| `server/public/model/config.go` | Added `BleveSettings` struct and wired into `Config.SetDefaults`/`IsValid` |
| `server/channels/app/platform/service.go` | Bleve engine initialization at startup |
| `server/channels/app/server.go` | Registers `BleveIndexerWorker` so `bleve_post_indexing` jobs can be created via API |
| `server/go.mod` | Added `github.com/blevesearch/bleve/v2 v2.4.1` |

### Search analyzer

The original v10 code used Bleve's `standard` analyzer (unicode tokenization + lowercase, no stemming).
This fork uses a custom `en_cs` analyzer instead:

- **Unicode tokenizer** — splits on word boundaries
- **Possessive filter** — strips English `'s`
- **Lowercase**
- **English stop words** — removes common words like "the", "is", "and"
- **Czech stop words** — removes common words like "a", "ale", "bez", "bylo"
- **Porter stemmer** — reduces words to their root ("jumping" → "jump", "running" → "run")

This gives proper stemming for English and stop-word filtering for both English and Czech.
Czech morphology is complex enough that Porter stemming won't cover all inflections, but the
combination is a significant improvement over plain substring matching for bilingual instances.

**Note:** the analyzer is baked into the index at creation time. If you upgrade from a version
of this fork that used the `standard` analyzer, you need to purge the existing indexes and
re-index so the new mapping takes effect.

The purge API endpoint (`/api/v4/bleve/purge_indexes`) was removed from this fork since the
`channels/api4/bleve.go` file was dropped. Purge manually by deleting the index directory on
the host and restarting the pod:

```bash
# 1. Delete index directory on the host (adjust path to match MM_BLEVESETTINGS_INDEXDIR)
ssh your-host "rm -rf /path/to/mattermost/data/bleve-indexes"

# 2. Restart the pod so Mattermost opens fresh indexes with the new mapping
kubectl rollout restart deployment/mattermost -n apps

# 3. Trigger bulk indexing for historical messages (requires a personal access token)
curl -X POST https://<your-mattermost-url>/api/v4/jobs \
  -H "Authorization: Bearer <your-token>" \
  -H "Content-Type: application/json" \
  -d '{"type":"bleve_post_indexing"}'
```

### Admin UI note

**There is no Bleve section in the System Console.** The Elasticsearch UI section is enterprise-only
and hidden in Team Edition. The Bleve config keys (`MM_BLEVESETTINGS_*`) work correctly via environment
variables — the UI just isn't there to configure or trigger them.

### Enabling Bleve

Set these environment variables (or the equivalent in `config.json`):

```
MM_BLEVESETTINGS_INDEXDIR=/mattermost/data/bleve-indexes
MM_BLEVESETTINGS_ENABLEINDEXING=true
MM_BLEVESETTINGS_ENABLESEARCHING=true
MM_BLEVESETTINGS_ENABLEAUTOCOMPLETE=true
```

### Bulk indexing existing messages

Since there's no UI to trigger indexing, use the API with a personal access token.

**Get a token:** click your avatar (top-right) → **Profile** → **Security** → **Personal Access Tokens** → Create.

```bash
curl -X POST https://<your-mattermost-url>/api/v4/jobs \
  -H "Authorization: Bearer <your-token>" \
  -H "Content-Type: application/json" \
  -d '{"type":"bleve_post_indexing"}'
```

New messages are indexed automatically as they arrive — the bulk job is only needed once for
historical data (and after any index purge).

### Upgrading to a new Mattermost version

```bash
git fetch upstream
git rebase v11.x.0   # conflicts unlikely — integration points are stable
git push origin bleve-restore --force-with-lease
```

---

# [![Mattermost logo](https://user-images.githubusercontent.com/7205829/137170381-fe86eef0-bccc-4fdd-8e92-b258884ebdd7.png)](https://mattermost.com)

[Mattermost](https://mattermost.com) is an open core, self-hosted collaboration platform that offers chat, workflow automation, voice calling, screen sharing, and AI integration. This repo is the primary source for core development on the Mattermost platform; it's written in Go and React, runs as a single Linux binary, and relies on PostgreSQL. A new compiled version is released under an MIT license every month on the 16th.

[Deploy Mattermost on-premises](https://mattermost.com/deploy/?utm_source=github-mattermost-server-readme), or [try it for free in the cloud](https://mattermost.com/sign-up/?utm_source=github-mattermost-server-readme).

<img width="1006" alt="mattermost user interface" src="https://user-images.githubusercontent.com/7205829/136107976-7a894c9e-290a-490d-8501-e5fdbfc3785a.png">

Learn more about the following use cases with Mattermost:

- [DevSecOps](https://mattermost.com/solutions/use-cases/devops/?utm_source=github-mattermost-server-readme)
- [Incident Resolution](https://mattermost.com/solutions/use-cases/incident-resolution/?utm_source=github-mattermost-server-readme)
- [IT Service Desk](https://mattermost.com/solutions/use-cases/it-service-desk/?utm_source=github-mattermost-server-readme)

Other useful resources:

- [Download and Install Mattermost](https://docs.mattermost.com/guides/deployment.html) - Install, setup, and configure your own Mattermost instance.
- [Product documentation](https://docs.mattermost.com/) - Learn how to run a Mattermost instance and take advantage of all the features.
- [Developer documentation](https://developers.mattermost.com/) - Contribute code to Mattermost or build an integration via APIs, Webhooks, slash commands, Apps, and plugins.

Table of contents
=================

- [Install Mattermost](#install-mattermost)
- [Native mobile and desktop apps](#native-mobile-and-desktop-apps)
- [Get security bulletins](#get-security-bulletins)
- [Get involved](#get-involved)
- [Learn more](#learn-more)
- [License](#license)
- [Get the latest news](#get-the-latest-news)
- [Contributing](#contributing)

## Install Mattermost

- [Download and Install Mattermost Self-Hosted](https://docs.mattermost.com/guides/deployment.html) - Deploy a Mattermost Self-hosted instance in minutes via Docker, Ubuntu, or tar.
- [Get started in the cloud](https://mattermost.com/sign-up/?utm_source=github-mattermost-server-readme) to try Mattermost today.
- [Developer machine setup](https://developers.mattermost.com/contribute/server/developer-setup) - Follow this guide if you want to write code for Mattermost.


Other install guides:

- [Deploy Mattermost on Docker](https://docs.mattermost.com/install/install-docker.html)
- [Mattermost Omnibus](https://docs.mattermost.com/install/installing-mattermost-omnibus.html)
- [Install Mattermost from Tar](https://docs.mattermost.com/install/install-tar.html)
- [Ubuntu 20.04 LTS](https://docs.mattermost.com/install/installing-ubuntu-2004-LTS.html)
- [Kubernetes](https://docs.mattermost.com/install/install-kubernetes.html)
- [Helm](https://docs.mattermost.com/install/install-kubernetes.html#installing-the-operators-via-helm)
- [Debian Buster](https://docs.mattermost.com/install/install-debian.html)
- [RHEL 8](https://docs.mattermost.com/install/install-rhel-8.html)
- [More server install guides](https://docs.mattermost.com/guides/deployment.html)

## Native mobile and desktop apps

In addition to the web interface, you can also download Mattermost clients for [Android](https://mattermost.com/pl/android-app/), [iOS](https://mattermost.com/pl/ios-app/), [Windows PC](https://docs.mattermost.com/install/desktop-app-install.html#windows-10-windows-8-1), [macOS](https://docs.mattermost.com/install/desktop-app-install.html#macos-10-9), and [Linux](https://docs.mattermost.com/install/desktop-app-install.html#linux).

[<img src="https://user-images.githubusercontent.com/30978331/272826427-6200c98f-7319-42c3-86d4-0b33ae99e01a.png" alt="Get Mattermost on Google Play" height="50px"/>](https://mattermost.com/pl/android-app/)  [<img src="https://developer.apple.com/assets/elements/badges/download-on-the-app-store.svg" alt="Get Mattermost on the App Store" height="50px"/>](https://itunes.apple.com/us/app/mattermost/id1257222717?mt=8)  [![Get Mattermost on Windows PC](https://user-images.githubusercontent.com/33878967/33095357-39cab8d2-ceb8-11e7-89a6-67dccc571ca3.png)](https://docs.mattermost.com/install/desktop.html#windows-10-windows-8-1-windows-7)  [![Get Mattermost on Mac OSX](https://user-images.githubusercontent.com/33878967/33095355-39a36f2a-ceb8-11e7-9b33-73d4f6d5d6c1.png)](https://docs.mattermost.com/install/desktop.html#macos-10-9)  [![Get Mattermost on Linux](https://user-images.githubusercontent.com/33878967/33095354-3990e256-ceb8-11e7-965d-b00a16e578de.png)](https://docs.mattermost.com/install/desktop.html#linux)

## Get security bulletins

Receive notifications of critical security updates. The sophistication of online attackers is perpetually increasing. If you're deploying Mattermost it's highly recommended you subscribe to the Mattermost Security Bulletin mailing list for updates on critical security releases.

[Subscribe here](https://mattermost.com/security-updates/#sign-up)

## Get involved

- [Contribute to Mattermost](https://handbook.mattermost.com/contributors/contributors/ways-to-contribute)
- [Find "Help Wanted" projects](https://github.com/mattermost/mattermost-server/issues?page=1&q=is%3Aissue+is%3Aopen+%22Help+Wanted%22&utf8=%E2%9C%93)
- [Join Developer Discussion on a Mattermost server for contributors](https://community.mattermost.com/signup_user_complete/?id=f1924a8db44ff3bb41c96424cdc20676)
- [Get Help With Mattermost](https://docs.mattermost.com/guides/get-help.html)

## Learn more

- [API options - webhooks, slash commands, drivers, and web service](https://api.mattermost.com/)
- [See who's using Mattermost](https://mattermost.com/customers/)
- [Browse over 700 Mattermost integrations](https://mattermost.com/marketplace/)

## License

See the [LICENSE file](LICENSE.txt) for license rights and limitations.

## Get the latest news

- **X** - Follow [Mattermost on X, formerly Twitter](https://twitter.com/mattermost).
- **Blog** - Get the latest updates from the [Mattermost blog](https://mattermost.com/blog/).
- **Facebook** - Follow [Mattermost on Facebook](https://www.facebook.com/MattermostHQ).
- **LinkedIn** - Follow [Mattermost on LinkedIn](https://www.linkedin.com/company/mattermost/).
- **Email** - Subscribe to our [newsletter](https://mattermost.us11.list-manage.com/subscribe?u=6cdba22349ae374e188e7ab8e&id=2add1c8034) (1 or 2 per month).
- **Mattermost** - Join the ~contributors channel on [the Mattermost Community Server](https://community.mattermost.com).
- **IRC** - Join the #matterbridge channel on [Freenode](https://freenode.net/) (thanks to [matterircd](https://github.com/42wim/matterircd)).
- **YouTube** -  Subscribe to [Mattermost](https://www.youtube.com/@MattermostHQ).

## Contributing

[![Small Image](https://img.shields.io/badge/Contribute%20with-Gitpod-908a85?logo=gitpod)](https://gitpod.io/#https://github.com/mattermost/mattermost)

Please see [CONTRIBUTING.md](./CONTRIBUTING.md).
[Join the Mattermost Contributors server](https://community.mattermost.com/signup_user_complete/?id=codoy5s743rq5mk18i7u5ksz7e) to join community discussions about contributions, development, and more.
