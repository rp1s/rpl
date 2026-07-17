# rpl:mongodb

`rpl:mongodb` generates BSON adapters, collection and index declarations,
search/sort allowlists, ObjectID conversion, CRUD, aggregation, distinct, bulk,
and change-stream helpers.

```rpl
@mongodb(db: "mongodb", collection: "products")
model Product {
    ID string @mongodb(objectId: true)
    Category string @mongodb(indexGroup: "category_price", indexOrder: 1)
    Price float64 @mongodb(indexGroup: "category_price", indexOrder: -1)
    Title string @mongodb(search: true, sort: true)
    UpdatedAt time.Time @mongodb(default: "now", updatedAt: true)
}
```

Fields sharing `indexGroup` become keys in one compound `bson.D` index in
declaration order. `indexOrder` accepts ascending `1` (the default) or
descending `-1`. `unique` and `sparse` on any grouped field configure the
resulting compound index.

Other field controls are `name`, `index`, `search`, `sort`, `objectId`,
`omitempty`, `default`, `updatedAt`, and `ignore`. Generated search and sort
allowlists prevent arbitrary document paths from reaching database queries.
