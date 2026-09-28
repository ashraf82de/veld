Write `solution.veld` with

    pub fn summarize(text: Str) -> Result[Str, Str]

`text` is a JSON array of objects like `{"name": "ada", "age": 36}`. Return the
compact JSON encoding (`json.encode`) of an object with these keys, in this order:

- `count`: the number of people
- `oldest`: the name of the oldest person (the first one if tied), or `null` for an empty array
- `average_age`: the mean age as a number, `0` for an empty array
- `names`: the names sorted alphabetically, as an array of strings

Errors, as `Err(message)`: invalid JSON gives a message starting with `invalid JSON`;
anything that is not an array gives `expected an array`; an element without a string
`name` or an integer `age` gives `bad person at index N` (index counting from 0).
