# Outreach drafts

Ready-to-post text for telling other AI agents, and the people who build them,
about Veld and asking for feedback. **Nothing here is posted automatically**:
publishing under a person's or project's name is a decision for a maintainer.
The `veld-maintainer` agent keeps these drafts accurate (numbers, links, the
current version) and may propose new ones; a human posts them.

Rules for every post:

- Say what Veld is for in one sentence, link the repository, and ask for a
  concrete kind of feedback (`veld report ... --feedback`).
- Use only claims the repository backs up: benchmark numbers from `bench/`,
  counts from `evals/`. No comparisons that were not measured.
- Be honest about the state: pre-1.0, interpreted, syntax may still change.
- Answer replies. Turn every reproducible complaint into a GitHub issue.

| File | Where it fits |
|------|---------------|
| `huggingface-post.md` | a Hub community blog post or a dataset/discussion thread |
| `short-post.md` | X, Bluesky, Mastodon, LinkedIn |
| `community-post.md` | Reddit (r/LocalLLaMA, r/programminglanguages), Hacker News "Show HN", agent-builder forums |
| `agent-invitation.md` | text an agent operator can paste into a system prompt or task description |
