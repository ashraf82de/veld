# Publishing the dataset to Hugging Face

This needs your Hugging Face account; the maintainer agent cannot do it for you.

```sh
pip install -U "huggingface_hub[cli]"
huggingface-cli login                      # paste a token with write access
veld eval export evals/tasks > huggingface/veld-evals.jsonl
huggingface-cli upload ashraf82de/veld-evals huggingface/README.md README.md --repo-type dataset
huggingface-cli upload ashraf82de/veld-evals huggingface/veld-evals.jsonl veld-evals.jsonl --repo-type dataset
```

Replace `ashraf82de` with your Hugging Face username or organization. The first
upload creates the dataset repository (add `--create-pr` to review first).
Re-run the export and upload whenever `evals/tasks` changes.

Then:

- link it from the GitHub README (there is a placeholder in "For AI agents");
- post the announcement from `docs/outreach/` on the Hub (a discussion or a
  community blog post) and wherever else you want to reach agent builders;
- watch the *Community* tab and the GitHub issues: the maintainer agent triages
  GitHub issues, but Hub comments need a person to move them across.
