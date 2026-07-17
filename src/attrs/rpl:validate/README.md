# rpl:validate

`rpl:validate` checks schema rules against field types and generates a typed
`Validate(model)` function with structured field errors.

```rpl
model Signup {
    ID string @validate(required: true, uuid: true)
    Login string @validate(required: true, pattern: "^[a-z][a-z0-9_]{2,39}$")
    Email string @validate(required: true, email: true, maxLen: 160)
    Tags []string @validate(minLen: 1, maxLen: 10)
    Birthday time.Time? @validate(past: true)
}
```

Rules include numeric/string `min` and `max`, `minLen`, `maxLen`, `required`,
`email`, `phone`, `url`, `uuid`, `pattern`, `past`, and sensitive-field `hash`
metadata. Format rules also support `[]string`; absent optional values are
skipped unless `required: true` is present.

Custom regular expressions are compiled during RPL analysis. Invalid patterns
and rules used on incompatible types fail generation with a field diagnostic.
