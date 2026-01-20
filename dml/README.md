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
  labels = [..., "new-label"]
WHERE id="4362f76c287a6866a1f1d1a206d8ad654ad84fc183a3f99a948eb60d1506918b";
```

Prepend entries in an existing list:

```
SET feedbacks
  labels = ["new-label", ...]
WHERE id="4362f76c287a6866a1f1d1a206d8ad654ad84fc183a3f99a948eb60d1506918b";
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
    Set ident AssignList Where Clause ";"

  DelStmt =
    Delete ident Where Clause ";"

  Set = 
    caseInsensitive<"SET">

  Delete = 
   caseInsensitive<"DELETE">
  
  Where = 
    caseInsensitive<"WHERE">

  AssignList = 
    Assign ("," AssignList)?

  Assign = 
    LFS "=" RFS

  LFS = 
  	"." | Traversal | ident

  RFS = 
    JSONValue | ArrayAppend | ArrayPrepend

  Clause = 
  	ident "=" Scalar

  Traversal  = 
  	ident "." (ident | String)+

  ident = 
  	letter+ (letter | "_" | "-")*

  ArrayAppend =
    "[" dotdotdot "," JSONValue ("," JSONValue)* "]"

  ArrayPrepend =
    "[" JSONValue ("," JSONValue)* "," dotdotdot "]"

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