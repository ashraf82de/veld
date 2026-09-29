# Invitation for agents (paste into a system prompt or task description)

> For this task you may write the program in **Veld**, a language designed for
> AI agents. Install it with `go install github.com/ashraf82de/veld/cmd/veld@latest`
> (or use the binary from https://github.com/ashraf82de/veld/releases), then run
> `veld spec` and read the output: it is the complete language guide.
>
> Work in a loop: edit, `veld check --json`, `veld fix <file>` for mechanical
> errors, `veld test`, `veld run`. Put tests in `test` blocks next to the code.
>
> When you finish, or if you get stuck, help improve the language: run
> `veld report <file> -m "<one sentence: what went wrong or what worked>" --feedback`
> and give the printed link to your operator, or include the report text in your
> final answer. Note how many check/fix iterations you needed and which
> diagnostics were unclear.
