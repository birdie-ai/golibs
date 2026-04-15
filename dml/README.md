# dml: Data Manipulation Language

This package provides the parser and encoder for the Data Manipulation Language (dml) used
as write interface for the search services.

## What is dml?

DML stands for [Data Manipulation Language](https://en.wikipedia.org/wiki/Data_manipulation_language) and this `dml` exists
primarily for three reasons:

1. Having a common language that captures the data change semantics independent of the transport protocol used (REST, PubSub, etc).
2. The ability to funnel data change events from different sources into a common language that allows for ease de-duplication and other optimizations.
3. Separate higher level business logic (mostly data transformation) from lower level storage modifications.

## Concepts

The DML works on top of the document-based data abstraction, which means
data is represented as an hierarchical data structure (like a JSON) and
then DML operations change internal parts of the documents.
The user doesn't need to know the details of the storage engine, if data
is stored as tables or files on disk, it doesn't matter and the only
surfacing concept is that documents are hierarchical and its constituint
parts are a subset of the JSON spec:

Primitive types:
- Number
- String
- Boolean

**NOTE: `null` is not supported, use DELETE operations for that**

Collection types:
- Array
- Object

Given that, let's say you have an entity called `operating_systems` which is defined as:

```json
{
	(name): (operating_system),
}
```

and `operating_system` has schema below:
```json
{
	"name": (string, pk=true),
	"description": (string, optional=true),
	"released_at": (date),
	"languages": [(string, set=true)], // a "set" means it has unique elements
	"authors": {
		(string): (author)
	}
}
```
Note that `authors` is not an array because we don't want duplicates, so author `name` is the primary key of this related entity.
 
Then schemas above means `operating_systems` is a map of `name` *->* `operating_system`.
The `author` has the schema below:
```json
{
	"id": (string, pk=true),
	"name": (string),
	"country": (string, optional=true)
}
```

Given the above, let's say we have this entity persisted with just the data below:

```json
{
	"EDSAC": {
		"name": "edsac",
		"languages": ["assembly"],
		"released_at": "1949-05-06",
		"authors": {
			"maurice-wilkes": {
				"id": "maurice-wilkes",
				"name": "Maurice Wilkes",
				"country": "United Kingdom"
			}
		}
	}
}
```

and user wants to update the _description_ that was not initially saved, then the DML statement
below can be used:

```sql
SET operating_systems
	description = "The Electronic Delay Storage Automatic Calculator (EDSAC) was an early British computer. Inspired by John von Neumann's seminal First Draft of a Report on the EDVAC, the machine was constructed by Maurice Wilkes and his team at the University of Cambridge Mathematical Laboratory in England to provide a service to the university.",
WHERE
	name="edsac";
```

Notice that `WHERE` clauses *must* address all fields marked as `pk` in the schema.

The current object is:
```json
{
	"edsac": {
		"name": "edsac",
		"description": "The Electronic Delay Storage Automatic Calculator (EDSAC) was an early British computer. Inspired by John von Neumann's seminal First Draft of a Report on the EDVAC, the machine was constructed by Maurice Wilkes and his team at the University of Cambridge Mathematical Laboratory in England to provide a service to the university.",
		"languages": ["assembly"],
		"released_at": "1949-05-06",
		"authors": {
			"maurice-wilkes": {
				"id": "maurice-wilkes",
				"name": "Maurice Wilkes",
				"country": "United Kingdom"
			}
		}
	}
}
```

Now let's say you want to add new operating systems, you can set whole objects at once with the `dot-assign` syntax, like the example below:

```
SET operating_systems
	.={
		"name": "plan9",
		"description": "Plan 9 from Bell Labs is an operating system designed by the Computing Science Research Center (CSRC) at Bell Labs in the mid-1980s, built on the UNIX concepts first developed there in the late 1960s. Since 2000, Plan 9 has been free and open-source. The final official release was in early 2015.",
		"languages": ["C"],
		"released_at": "1992-09-21",
		"authors": {
			"rob-pike": {
				"id": "rob-pike",
				"name": "Rob Pike",
				"country": "United States"
			},
			"ken-thompson": {
				"id": "ken-thompson",
				"name": "Ken Thompson",
				"country": "United States"
			},
			"dave-presotto": {
				"id": "dave-presotto",
				"name": "Dave Presotto",
				"country": "United States"
			},
			"phil-winterbottom": {
				"id": "phil-winterbottom",
				"name": "Phil Winterbotton"
			}
		}
	}
WHERE name="plan9";
```

The reason that we do a `SET` for both *inserting* and *updating* is because all DML statements
are *idempotent* and this is a very important property of the Birdie data change events.

Once statement above executes, the persisted entity will be:

```json
{
	"edsac": {
		"name": "edsac",
		"description": "The Electronic Delay Storage Automatic Calculator (EDSAC) was an early British computer. Inspired by John von Neumann's seminal First Draft of a Report on the EDVAC, the machine was constructed by Maurice Wilkes and his team at the University of Cambridge Mathematical Laboratory in England to provide a service to the university.",
		"languages": ["assembly"],
		"released_at": "1949-05-06",
		"authors": {
			"maurice-wilkes": {
				"id": "maurice-wilkes",
				"name": "Maurice Wilkes",
				"country": "United Kingdom"
			}
		}
	},
	"plan9": {
		"name": "plan9",
		"description": "Plan 9 from Bell Labs is an operating system designed by the Computing Science Research Center (CSRC) at Bell Labs in the mid-1980s, built on the UNIX concepts first developed there in the late 1960s. Since 2000, Plan 9 has been free and open-source. The final official release was in early 2015.",
		"languages": ["C"],
		"released_at": "1992-09-21",
		"authors": {
			"rob-pike": {
				"id": "rob-pike",
				"name": "Rob Pike",
				"country": "United States"
			},
			"ken-thompson": {
				"id": "ken-thompson",
				"name": "Ken Thompson",
				"country": "United States"
			},
			"dave-presotto": {
				"id": "dave-presotto",
				"name": "Dave Presotto",
				"country": "United States"
			},
			"phil-winterbottom": {
				"id": "phil-winterbottom",
				"name": "Phil Winterbotton"
			}
		}
	}
}
```

If you are an afficionado about UNIX/Bell-Labs/Plan9 history you should notice an error in the
data above: Rob Pike was born in Canada...

To fix that, we have to fix an entry inside the nested "authors" entity, and for that we have to introduce the concept of *nested DML statements*. 

```sql
SET operating_systems
	(
		SET authors country="Canada" WHERE id="rob-pike"
	)
WHERE name="plan9";
```

The *stmt* is recursive and children statement works inside the relationships.
This way its `WHERE` clauses are clear and **unambiguous**.

The final object must be:
```json
{
	"edsac": {
		"name": "edsac",
		"description": "The Electronic Delay Storage Automatic Calculator (EDSAC) was an early British computer. Inspired by John von Neumann's seminal First Draft of a Report on the EDVAC, the machine was constructed by Maurice Wilkes and his team at the University of Cambridge Mathematical Laboratory in England to provide a service to the university.",
		"languages": ["assembly"],
		"released_at": "1949-05-06",
		"authors": {
			"maurice-wilkes": {
				"id": "maurice-wilkes",
				"name": "Maurice Wilkes",
				"country": "United Kingdom"
			}
		}
	},
	"plan9": {
		"name": "plan9",
		"description": "Plan 9 from Bell Labs is an operating system designed by the Computing Science Research Center (CSRC) at Bell Labs in the mid-1980s, built on the UNIX concepts first developed there in the late 1960s. Since 2000, Plan 9 has been free and open-source. The final official release was in early 2015.",
		"languages": ["C"],
		"released_at": "1992-09-21",
		"authors": {
			"rob-pike": {
				"id": "rob-pike",
				"name": "Rob Pike",
				"country": "United States"
			},
			"ken-thompson": {
				"id": "ken-thompson",
				"name": "Ken Thompson",
				"country": "United States"
			},
			"dave-presotto": {
				"id": "dave-presotto",
				"name": "Dave Presotto",
				"country": "United States"
			},
			"phil-winterbottom": {
				"id": "phil-winterbottom",
				"name": "Phil Winterbotton"
			}
		}
	}
}
```

As we all know, _Russ Cox_ had an important role in the evolution of _Plan9_, then it's fair to
add him to the authors. Additionally, we know that Plan9 also has parts of the kernel written
in assembly, so let's append it to the languages *set* as well:

```
SET operating_systems
	languages=...["assembly"],
	"authors.russ-cox"={
		"id": "russ-cox",
		"name": "Russ Cox",
		"country": "United States"
	}
WHERE name="plan9";
```

Alternatively, the statement above can be expressed as:
```
SET operating_systems
	languages=...["assembly"],
	(
		SET authors
			.={
				"id": "russ-cox",
				"name": "Russ Cox",
				"country": "United States"
			}
		WHERE id="russ-cox"
	}
WHERE name="plan9";
```

If you followed all steps, then the final object is:

```json
{
	"edsac": {
		"name": "edsac",
		"description": "The Electronic Delay Storage Automatic Calculator (EDSAC) was an early British computer. Inspired by John von Neumann's seminal First Draft of a Report on the EDVAC, the machine was constructed by Maurice Wilkes and his team at the University of Cambridge Mathematical Laboratory in England to provide a service to the university.",
		"languages": ["assembly"],
		"released_at": "1949-05-06",
		"authors": {
			"maurice-wilkes": {
				"id": "maurice-wilkes",
				"name": "Maurice Wilkes",
				"country": "United Kingdom"
			}
		}
	},
	"plan9": {
		"name": "plan9",
		"description": "Plan 9 from Bell Labs is an operating system designed by the Computing Science Research Center (CSRC) at Bell Labs in the mid-1980s, built on the UNIX concepts first developed there in the late 1960s. Since 2000, Plan 9 has been free and open-source. The final official release was in early 2015.",
		"languages": ["assembly", "C"],
		"released_at": "1992-09-21",
		"authors": {
			"rob-pike": {
				"id": "rob-pike",
				"name": "Rob Pike",
				"country": "United States"
			},
			"ken-thompson": {
				"id": "ken-thompson",
				"name": "Ken Thompson",
				"country": "United States"
			},
			"dave-presotto": {
				"id": "dave-presotto",
				"name": "Dave Presotto",
				"country": "United States"
			},
			"phil-winterbottom": {
				"id": "phil-winterbottom",
				"name": "Phil Winterbotton"
			},
			"russ-cox": {
				"id": "russ-cox",
				"name": "Russ Cox",
				"country": "United States"
			}
		}
	}
}
```

See more examples below:

## Examples

### Update data

Set individual fields:
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

Set an entire object field:

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

Set all fields with an object:
```
SET feedbacks
  . = {
    "text": "some feedback text",
    "custom_fields": {},
    "entity": "feedback"
  }
WHERE id="4362f76c287a6866a1f1d1a206d8ad654ad84fc183a3f99a948eb60d1506918b";
```

Append entries in an existent list:

```
SET feedbacks
  labels = ... ["new-label"]
WHERE id="4362f76c287a6866a1f1d1a206d8ad654ad84fc183a3f99a948eb60d1506918b";
```

Prepend entries in an existing list:

```
SET feedbacks
  labels = ["new-label"] ...
WHERE id="4362f76c287a6866a1f1d1a206d8ad654ad84fc183a3f99a948eb60d1506918b";
```

### Delete data

Delete an entity record:

```
DELETE conversations . WHERE id="abc";
```

Assuming `custom_fields` is an object, then stmt below delete a dynamic field from an
entity record (the record is not deleted, just the field):

```
DELETE conversations
	custom_fields.country
WHERE id="abc";
```

Deleting a list of fields:
```
DELETE conversations
	custom_fields[k] : k IN ["a","b"]
WHERE id="abc";
```

The statement above reads as `FOR EACH key k IN custom_fields WHERE k IS IN THE LIST ["a","b"]`.

For deleting based on field name and value condition:
```
DELETE conversations
	custom_fields[k] => v : k="country" AND v="us"
WHERE id="abc";
```

The statement above reads as `FOR EACH KEY k AND VALUE v IN custom_fields WHERE k EQUALS "country" AND v EQUALS "us"`.

If the key is not needed, it can be omitted with `_`.

```
DELETE conversations
	custom_fields[_] => v : v = "something"
WHERE id="abc";
```

If `labels` has the schema below:
```
[ (string) ]
```

then stmt below deletes "label-1" and "label-2":

```
DELETE conversations
	labels[_] as v : v IN ["label-1", "label-2"]
WHERE id="abc";
```

All examples above can be grouped into a single DELETE stmt:

```
DELETE conversations
	custom_fields.abc,
	custom_fields[k] as v : k="country" AND v="us",
	labels[_] as l : l="label-1",
WHERE id="abc";
```

## Nested Stmts

Set `feedbacks.text` inside the `orders` entity:

```
SET orders
	(
		SET feedbacks
			text="some text",
		WHERE id="abc"
	)
WHERE id="order_id";
```

## Syntax

The language grammar is defined below using [ohm](https://ohmjs.org/docs/syntax-reference).
You can paste the code below in the [online editor](https://ohmjs.org/editor/) to
validate the syntax definition by providing good/bad examples.

```
dml {
  Stmts = 
    Stmt*

  Stmt =
    SetStmt   -- setstmt
    | DelStmt -- delstmt

  SetStmt = 
    Set ident AssignList Where Condition ";"

  DelStmt =
    Delete ident DeleteFilter? Where Condition ";"

  Set = 
    caseInsensitive<"SET">

  Delete = 
   caseInsensitive<"DELETE">
  
  Where = 
    caseInsensitive<"WHERE">
    
  In =
  	caseInsensitive<"IN">
    
  Arrow =
  	"=>"

  AssignList = 
    Assign ("," AssignList)?

  Assign = 
    LFS "=" RFS
    
  DeleteFilter =
  	LFS (KeyDecl VarDecl? ":" Condition)?
    
  KeyDecl =
  	"[" (ident | "_") "]"
    
  VarDecl =
  	Arrow ident
    
  LFS = 
  	"." | Traversal | ident

  RFS = 
    ArrayAppend | ArrayPrepend | JSONValue
    
  Condition =
  	Clause (LogicalOp Clause)?
    
  LogicalOp = AND | OR
  
  AND = 
  	caseInsensitive<"AND">
    
  OR =
  	caseInsensitive<"OR">

  Clause = 
  	ident "=" Scalar

  Traversal  = 
  	ident "." (ident | String)+

  ident = 
  	letter+ (letter | "_" | "-")*

  dotdotdot =
	"..."

  ArrayAppend =
    dotdotdot Array

  ArrayPrepend =
    Array dotdotdot

  JSONValue =
    Object
    | Array
    | String
    | Number
    | True
    | False
    | Null
    
    Scalar =
		String | Number | True | False

  Object =
    "{" "}" -- empty
    | "{" Pair ("," Pair)* "}" -- nonEmpty

  Pair =
    String ":" JSONValue

  Array =
    "[" "]" -- empty
    | "[" JSONValue ("," JSONValue)* "]" -- nonEmpty

  String (String) =
    stringLit

  stringLit =
    "\"" doubleStringCharacter* "\""

  doubleStringCharacter (character) =
    ~("\"" | "\\") any -- nonEscaped
    | "\\" escapeSequence -- escaped

  escapeSequence =
    "\"" -- doubleQuote
    | "\\" -- reverseSolidus
    | "/" -- solidus
    | "b" -- backspace
    | "f" -- formfeed
    | "n" -- newline
    | "r" -- carriageReturn
    | "t" -- horizontalTab
    | "u" fourHexDigits -- codePoint

  fourHexDigits = hexDigit hexDigit hexDigit hexDigit

  Number (Number) =
    numberLit

  numberLit =
    decimal exponent -- withExponent
    | decimal -- withoutExponent

  decimal =
    wholeNumber "." digit+ -- withFract
    | wholeNumber -- withoutFract

  wholeNumber =
    "-" unsignedWholeNumber -- negative
    | unsignedWholeNumber -- nonNegative

  unsignedWholeNumber =
    "0" -- zero
    | nonZeroDigit digit* -- nonZero

  nonZeroDigit = "1".."9"

  exponent =
    exponentMark ("+"|"-") digit+ -- signed
    | exponentMark digit+ -- unsigned

  exponentMark = "e" | "E"

  True = "true"
  False = "false"
  Null = "null"
}
```