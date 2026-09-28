Write `solution.veld` with

    pub fn route(method: Str, path: Str) -> Str

that routes an HTTP request and returns the name of the handler:

| method | path            | result              |
|--------|-----------------|---------------------|
| GET    | `/`             | `home`              |
| GET    | `/users`        | `list users`        |
| POST   | `/users`        | `create user`       |
| GET    | `/users/<id>`   | `show user <id>`    |
| DELETE | `/users/<id>`   | `delete user <id>`  |

Other methods on those paths return `405`. Every other path returns `404`. A
trailing slash makes no difference (`/users/` is `/users`), and `<id>` is any
single non-empty path segment.
