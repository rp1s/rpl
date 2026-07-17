# validate examples

The validation plugin emits a typed `Validate(model)` function and structured
field errors. Rules are checked against field types during RPL analysis.

| Example | Features |
| --- | --- |
| `01-string-and-number-validation.rpl` | numeric ranges, email, and phone |
| `02-length-time-and-hash.rpl` | length bounds, URL, past time, and hash policy |
| `03-collections-and-optional-values.rpl` | list length, list item email/URL validation, optional values |
| `04-security-profile.rpl` | combined account-security rules and `argon2id` metadata |

`min`/`max` validate scalar numeric values or string size. `minLen`/`maxLen`
validate strings and collections. `email`, `phone`, and `url` also work on
`[]string`, validating every item. Optional values are skipped when absent and
validated normally when present.
