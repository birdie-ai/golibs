# dml: Data Manipulation Language

This package provides the parser and encoder for the Data Manipulation Language (dml) used
as write interface for the search services.

## What is dml?

DML stands for [Data Manipulation Language](https://en.wikipedia.org/wiki/Data_manipulation_language) and this `dml` exists
primarily for three reasons:

1. Having a common language that captures the data change semantics independent of the transport protocol used (REST, PubSub, etc).
2. The ability to funnel data change events from different sources into a common language that allows for ease de-duplication and other optimizations.
3. Separate higher level business logic (mostly data transformation) from lower level storage modifications.

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
DELETE conversations WHERE id="abc";
```

Assuming `custom_fields` is an object, then stmt below delete a dynamic field from an
entity record (the record is not deleted, just the field):

```
DELETE conversations
	custom_fields.country
WHERE id="abc";
```

Alternatively, the syntax below does the same but allows for fields with spaces:

```
DELETE conversations
	custom_fields["country"]
WHERE id="abc";
```

and again alternatively, you can use the syntax below for deleting a list of fields:
```
DELETE conversations
	custom_fields[k] : k IN ["a","b"]
WHERE id="abc";
```

The statement above reads as `FOR EACH key k IN custom_fields WHERE k IS IN THE LIST ["a","b"]`.

For deleting based on field name and value condition:
```
DELETE conversations
	custom_fields[k] => v WHERE k="country" AND v="us"
WHERE id="abc";
```

The statement above reads as `FOR EACH KEY k AND VALUE v IN custom_fields WHERE k EQUALS "country" AND v EQUALS "us"`.

If the key is not needed, it can be omitted with `_`.

```
DELETE conversations
	custom_fields[_] => v WHERE v = "something"
WHERE id="abc";
```

If `labels` has the schema below:
```
[ (string) ]
```

then stmt below deletes "label-1" and "label-2":

```
DELETE conversations
	labels[_] as v WHERE v IN ["label-1", "label-2"]
WHERE id="abc";
```

All examples above can be grouped into a single DELETE stmt:

```
DELETE conversations
	custom_fields.abc,
	custom_fields[k] as v WHERE k="country" AND v="us",
	labels[_] as l WHERE l="label-1",
WHERE id="abc";
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