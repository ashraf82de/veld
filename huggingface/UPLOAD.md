# Publishing the dataset to Hugging Face

This needs an authenticated Hugging Face account with dataset write access.
No such write capability is currently available to the maintainer, so the
commands below are instructions rather than evidence of publication. Follow
the current official [Hub CLI guide](https://huggingface.co/docs/huggingface_hub/guides/cli).

```sh
pip install -U "huggingface_hub[cli]"
hf auth login                              # authenticate with write access
veld eval export evals/tasks > huggingface/veld-evals.jsonl
hf upload ashraf82de/veld-evals huggingface/README.md README.md --repo-type dataset
hf upload ashraf82de/veld-evals huggingface/veld-evals.jsonl veld-evals.jsonl --repo-type dataset
```

Replace `ashraf82de` with your Hugging Face username or organization. The first
upload creates the dataset repository (add `--create-pr` to review first).
Re-run the export and upload whenever `evals/tasks` changes.
After uploading, open the returned public dataset URL and verify both files
before recording or announcing the publication.

Then:

- link it from the GitHub README (there is a placeholder in "For AI agents");
- post the announcement from `docs/outreach/` on the Hub (a discussion or a
  community blog post) and wherever else you want to reach agent builders;
- watch the *Community* tab and the GitHub issues: the maintainer agent triages
  GitHub issues, but Hub comments need a person to move them across.
