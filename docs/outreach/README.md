# Outreach drafts

These are reviewed drafts for inviting agent developers and model evaluators
to try Veld. A file in this directory is not evidence of external publication.

The owner authorizes the maintainer to publish relevant invitations when an
authenticated publishing capability is available. Follow
[the maintainer policy](../MAINTAINING.md); do not create repeated promotional
posts or unrelated comments on other projects.

## Publication status (checked 2026-10-02)

No Hugging Face post or dataset publication has been verified.
The connected account is `ashraf82de`; its connector has `read-repos` but no
repository write scope or post-publishing tool. It reports a non-PRO account.

Hugging Face documents [social posts as a PRO feature](https://huggingface.co/docs/hub/pro).
[Personal blog articles](https://huggingface.co/docs/hub/blog-articles) require
a confirmed email plus PRO or a qualifying Team/Enterprise membership.
Check current account eligibility before publishing. Repository write access
and permission to publish social posts are separate capabilities.

After a successful publication, record its verified public URL and date here.
Until then, share the GitHub guide and eval links; do not invent a Hub URL.

## Editorial rules

- Describe implemented features separately from design goals.
- No model-performance comparison without a recorded evaluation.
- Reference solutions passing is evidence about the evaluator, not an AI model.
- Runtime numbers need a source commit, hardware, command and methodology.
- Be clear that Veld is pre-1.0 and interpreted.
- Ask for model/version, task, source commit, first-attempt results, repair
  iterations and a minimal reproducer. Review reports for private data.
- Withhold public tests/reference answers from the model during evaluation.
- Respond to relevant replies and link reproducible reports to GitHub issues.

| File | Where it fits |
|------|---------------|
| `huggingface-post.md` | Longer Hub article or a relevant project discussion |
| `short-post.md` | A short Hub social post or other social announcement |
| `community-post.md` | Agent-builder forums; review before use |
| `agent-invitation.md` | A task prompt an agent operator can copy |
