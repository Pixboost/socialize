# socialize

Turn existing material into a connected, long-running social content series.
Socialize is planned as a configuration-driven CLI that analyzes source
material, generates an editable publishing plan, and publishes due posts to
configured channels. The first use case is LinkedIn content for Pixboost, with
YouTube transcripts as a potential source; the tool is not limited to technical
content.

**Status:** Planning, with an experimental LinkedIn draft CLI. The Socialize
commands and configuration below describe the intended interface and are not
implemented yet.

## LinkedIn draft experiment

This branch contains a small Go CLI that attempts to create a static,
unpublished draft using LinkedIn's older `/v2/ugcPosts` API and its `DRAFT`
lifecycle state. It looks up the authenticated member using OpenID Connect
`/v2/userinfo` and sets that person as the author, not the app. A published
post would be attributed to that person, though the exact presentation and
whether the app is shown are determined by LinkedIn.

**This is an experiment:** access to draft creation, and whether API-created
drafts appear in LinkedIn's UI, have not been verified. LinkedIn may reject
the request or require access beyond a newly created app's permissions. The
tool never falls back to publishing a public post. The `PUBLIC` visibility in
the request is part of the post payload; the requested lifecycle state is
always `DRAFT`.

Prerequisites:

- Go 1.22 or later.
- A LinkedIn developer app with access to **Share on LinkedIn** and **Sign In
  with LinkedIn using OpenID Connect**, subject to LinkedIn's current product
  and approval requirements.
- A member OAuth access token for the account that should be the author, with
  `w_member_social`, `openid`, and `profile` scopes. Obtain it through
  LinkedIn's OAuth flow or developer tools; this experiment does not implement
  OAuth or token refresh.

Set the token in your environment without committing it, then run:

```sh
export LINKEDIN_ACCESS_TOKEN='your-member-access-token'
go run ./cmd/linkedin-draft
```

The CLI prints the member URN and post ID if LinkedIn accepts the request.
Check your LinkedIn account to see whether it appears as a draft. If the
request times out or returns an ambiguous result, check the account before
retrying; the request may have been accepted. Do not share the token or commit
it to the repository.

## Intended workflow

1. Configure the project, sources, audience, channel, and posting cadence.
2. Run `socialize plan` to create a series of connected, source-grounded drafts
   and assign publication dates. Review and edit the resulting plan.
3. Run `socialize publish` on a recurring schedule. It publishes only eligible,
   unpublished posts whose publication date has arrived, then records the
   result in the plan.

For example, a 26-week plan with two posts per week would contain approximately
52 posts. Rather than producing isolated summaries, the series should develop
themes over time, connect related posts, and link back to relevant sources.

### Preview

`socialize plan --dry-run` should generate a small sample of undated drafts for
review, without scheduling or publishing anything. This is a real generation
run: it may read sources, make network and AI-provider calls, incur costs, and
save a local preview. It must not change the production plan.

### Illustrative configuration

The schema and command flags are provisional:

```yaml
project: Pixboost

sources:
  - type: files
    path: ./sources
  - type: youtube_transcripts
    path: ./transcripts

channels:
  linkedin:
    audience: People interested in faster, better-performing websites
    tone: helpful, approachable
    schedule:
      start: "2027-01-04"
      timezone: Europe/London
      days: [monday, thursday]
      time: "09:00"
      weeks: 26

dry_run:
  sample_size: 3
```

```sh
socialize plan --config config.yaml --dry-run
socialize plan --config config.yaml
socialize publish --plan plan.yaml
```

An illustrative entry in the editable plan:

```yaml
posts:
  - id: website-speed-01
    channel: linkedin
    publish_date: "2027-01-04T09:00:00+00:00"
    status: planned
    content: |
      Your LinkedIn post goes here.
    sources:
      - https://example.com/blog/website-speed
```

After a confirmed publication, the plan should record `published` status, the
actual publication time, and the platform's post identifier. Publication
failures must remain visible; ambiguous outcomes need reconciliation rather than
blind retries that could create duplicates.

## Implementation plan

- [ ] Define and validate the project configuration and editable plan formats,
      including stable post IDs, source references, channel settings, time zones,
      publication state, and relationships between posts.
- [ ] Ingest local text, Markdown, source code, and transcript files; support
      public blog and documentation URLs. Accept existing LinkedIn posts as
      supplied text or files rather than relying on scraping.
- [ ] Analyze sources into themes and claims with traceable references. Flag
      unsupported claims for review, and enable users to exclude source material.
- [ ] Generate a coherent series and calendar from the configured cadence,
      keeping drafts editable and preserving source and cross-post references.
- [ ] Implement dry-run generation of a configurable number of undated sample
      drafts, isolated from the production plan and publication workflow.
- [ ] Implement LinkedIn publishing for due posts using the appropriate API
      permissions. Persist each confirmed result, expose failures, and design
      for concurrent runs and ambiguous API responses to avoid duplicate posts.
- [ ] Support reliable recurring execution through an external scheduler, such
      as cron; document persistent plan storage and operational requirements.
- [ ] Document provider configuration, credentials, data handling, and setup.
      Keep credentials out of configuration and plan files, and clearly explain
      what source content is sent to external providers.
- [ ] Add tests for source handling, plan generation, dry-run isolation,
      scheduling and time zones, publication state, retries, and failure cases.

LinkedIn publishing requires authorization and applicable API permissions;
access may differ for personal profiles and organization pages. LinkedIn does
not provide the scheduling workflow assumed here: a recurring process must run
`publish` when posts are due. YouTube output and additional platforms are future
possibilities, not part of the initial publishing scope.

## License

[GNU General Public License v3.0 only](LICENSE). Commercial use is permitted;
distribution of modified versions is subject to the license's terms.
