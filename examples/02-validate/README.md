# validate examples

The validation plugin emits a typed `Validate(model)` function and structured
field errors. Rules are checked against field types during RPL analysis.

| Example | Features |
| --- | --- |
| `01-string-and-number-validation.rpl` | numeric ranges, email, and phone |
| `02-length-time-and-hash.rpl` | length bounds, URL, past time, and hash policy |
| `03-collections-and-optional-values.rpl` | list length, list item email/URL validation, optional values |
| `04-security-profile.rpl` | required values, UUID, custom regexp, and `argon2id` metadata |

`min`/`max` validate scalar numeric values or string size. `minLen`/`maxLen`
validate strings and collections. `email`, `phone`, and `url` also work on
`[]string`, validating every item. Optional values are skipped when absent and
validated normally when present.

`required: true` rejects empty strings, empty collections, missing optional
values, and zero timestamps. `uuid: true` accepts canonical UUID strings.
`pattern` is compiled while RPL analyzes the schema, so an invalid regular
expression fails the build instead of becoming a runtime surprise.
