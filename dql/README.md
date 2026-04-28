# dql

`dql` (Birdie **D**ata **Q**uery **L**anguage) is a text-based query language.
It gives a consistent language for querying Birdie entities abstracting away
the storage engine used.

## Language

Example:

```sql
AS fraud_feedbacks SEARCH feedbacks id, text
 WHERE {
   "$and": [
     {"$or": [ {"text": "fraud"}, {"labels": "opportunity:10:true"} ]},
     {"posted_at": {"$gte": "2025-01-01T00:00:00Z"}}
   ]
 }
 LIMIT 50
 AGGS {
   by_label: terms("labels"),
   by_month: date_histogram(posted_at, {"interval": "month"})
 };

RETURN {
	"results": fraud_feedbacks.docs,
	"metrics": {
		"labels_count": fraud_feedbacks.aggs.by_label,
		"monthly_frauds": fraud_feedbacks.aggs.by_month,
	}
};
```

### SEARCH statement

A `SEARCH` statement allows for searching/aggregating data in a flexible way. A program can have an
undefinite number of such statements.

Example:
```
SEARCH operating_systems;
```
This returns up to **default size limit** of documents from the search engine. The
default size limit is outside the scope of the language and is defined by the
search engine you are communicating with.

Example output:
```json
[
	{
		"docs": [
			{
				"name": "edsac",
				"description": "The Electronic Delay Storage Automatic Calculator (EDSAC) was an early British computer. Inspired by John von Neumann's seminal First Draft of a Report on the EDVAC, the machine was constructed by Maurice Wilkes and his team at the University of Cambridge Mathematical Laboratory in England to provide a service to the university.",
				"languages": ["assembly"],
				"released_at": "1949-05-06",
				"authors": [
					{
						"id": "maurice-wilkes",
						"name": "Maurice Wilkes",
						"country": "United Kingdom"
					}
				]
			},
			{
				"name": "plan9",
				"description": "Plan 9 from Bell Labs is an operating system designed by the Computing Science Research Center (CSRC) at Bell Labs in the mid-1980s, built on the UNIX concepts first developed there in the late 1960s. Since 2000, Plan 9 has been free and open-source. The final official release was in early 2015.",
				"languages": ["C"],
				"released_at": "1992-09-21",
				"authors": [
					{
						"id": "rob-pike",
						"name": "Rob Pike",
						"country": "United States"
					},
					{
						"id": "ken-thompson",
						"name": "Ken Thompson",
						"country": "United States"
					},
					{
						"id": "dave-presotto",
						"name": "Dave Presotto",
						"country": "United States"
					},
					{
						"id": "phil-winterbottom",
						"name": "Phil Winterbotton"
					}
				]
			}
		]
	}
]
```

By default all fields are returned.

### Fields (projection)

For performance reasons, it's advisable that just the needed fields are returned. 
Example:

```
SEARCH operating_systems name, description;
```

This returns:
```
[
	{
		"docs": [
			{
				"name": "edsac",
				"description": "The Electronic Delay Storage Automatic Calculator (EDSAC) was an early British computer. Inspired by John von Neumann's seminal First Draft of a Report on the EDVAC, the machine was constructed by Maurice Wilkes and his team at the University of Cambridge Mathematical Laboratory in England to provide a service to the university.",
			},
			{
				"name": "plan9",
				"description": "Plan 9 from Bell Labs is an operating system designed by the Computing Science Research Center (CSRC) at Bell Labs in the mid-1980s, built on the UNIX concepts first developed there in the late 1960s. Since 2000, Plan 9 has been free and open-source. The final official release was in early 2015."
			}
		]
	}
]
```

The fields can be paths like `obj.name`. Example:

```
SEARCH operating_systems authors.name
```

Which returns:
```
[
	{
		"docs": [
			{
				"authors": [
					{
						"name": "Maurice Wilkes",
					}
				]
			},
			{
				"authors": [
					{
						"name": "Rob Pike",
					},
					{
						"name": "Ken Thompson",
					},
					{
						"name": "Dave Presotto",
					},
					{
						"name": "Phil Winterbotton"
					}
				]
			}
		]
	}
]
```

### Named statements: `AS`

A statement can be bound to a name for use by later clauses using the `AS` keyword.
It's usage is `AS <name> SEARCH ...` which names the `SEARCH` statement and make its result
available in other `SEARCH` statements or in the `RETURN` statement.

Example:
```sql
AS happy_accounts SEARCH feedbacks account.id WHERE text ~= "I love this product" LIMIT 100;
AS happy_feedbacks SEARCH feedbacks id WHERE account.id = happy_accounts.docs[*].account.id;
```

Note that in above program, the second stmt does `account.id = happy_accounts.docs[*].account.id`.
This creates a dependency which makes `happy_accounts` statement execute before `happy_feedbacks`.

### Filter: `WHERE` clause

TBD (we support the internal Birdie search DSL)

### LIMIT

```
SEARCH operating_systems name LIMIT 1;
```

Returns:
```
[
	{
		"docs": [
			{
				"name": "edsac"
			}
		]
	}
]
```

### Aggregations

Example:

```
SEARCH operating_systems LIMIT 0 AGGS {
	by_language: terms(language)
}
```

Returns: 
```
[
	{
		"aggs": {
			"by_language": {
				"buckets": [
					{
						"doc_count": 3,
						"key": "C"
					},
					{
						"doc_count": 1,
						"key": "assembly"
					}
				]
			}
		}
	}
]
```
