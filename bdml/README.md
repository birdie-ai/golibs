# bdml: Birdie Data Manipulation Language

This package provides the parser and encoder for the Birdie Data Manipulation
Language (bdml).

## What is bdml?

The `bdml` language exists for three reasons:

1. Having a common language that captures the data change semantics independent of the transport protocol used (REST, PubSub, etc).
2. The ability to funnel data change events from different sources into a common language that allows for ease de-duplication and other optimizations.
3. Separate higher level business logic (mostly data transformation) from lower level storage modifications.

## Syntax

Simplified eBNF:
```
PROGRAM = STMT+
STMT = OP IDENT [ASSIGNLIST] 'WHERE' CLAUSE ';'
OP = 'SET' | 'DELETE'
ASSIGNLIST = NAME '=' VALUE [',' ASSIGNLIST ]
CLAUSE = NAME '=' VALUE
NAME = '.' | IDENT ['.' IDENT]
VALUE = JSON
IDENT = #'\w+'
JSON = '"' IDENT '"'
```

Examples:

Updating individual fields:
```
SET feedbacks
  custom_fields.connector_id = {
      "description": "Connector id",
      "value": "a0aca3a3-43d7-4c8d-9090-d4594b46e458",
      "type": "text",
      "repeated": false
  }
WHERE id="4362f76c287a6866a1f1d1a206d8ad654ad84fc183a3f99a948eb60d1506918b";
```

Update an entire object field:

```
SET feedbacks
  custom_fields = {
    "connector_id" = {
      "description": "Connector id",
      "value": "a0aca3a3-43d7-4c8d-9090-d4594b46e458",
      "type": "text",
      "repeated": false
    },
    "abc" = {
      "description": "ABC field",
      "value": "some value",
      "type": "text",
      "repeated": false
    }
  }
WHERE id="4362f76c287a6866a1f1d1a206d8ad654ad84fc183a3f99a948eb60d1506918b";
```

```
SET feedbacks
  kind.rating=10,
  text="some feedback text"
WHERE id="4362f76c287a6866a1f1d1a206d8ad654ad84fc183a3f99a948eb60d1506918b";
```

Updating whole object:
```
SET feedbacks
  . = {
    "text": "some feedback text",
    "custom_fields": {},
    "entity": "feedback"
  }
WHERE id="4362f76c287a6866a1f1d1a206d8ad654ad84fc183a3f99a948eb60d1506918b";
```
