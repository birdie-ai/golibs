# Katschema

Katschema is a minimalist (yet powerful) schema language for defining the structure
and constraints of data using its shape.

Instead of describing schemas through verbose declarations, Katschema mirrors the
structure of the data itself. This makes schemas easy to read, write, and reason
about while still being expressive.

It supports the features below:

- primitive and custom types
- type constraint expressions
- optionality
- builtin functions

For example, if you have the data below:
```json
{
	"id": 137,
	"name": "Richard Feynman",
	"interests": ["physics", "electronics", "lockpicking", "painting"]
}
```
you can define its schema with:
```json
{
	"id": (int),
	"name": (string),
	"interests": ([ (string) ])
}
```

## Primitive types

The types below are builtin:

- `string`: UTF-8 string.
- `number`: 64-bit floating point.
- `bool`: `true` or `false`
- `int`: variable size integer

## Collection types

- `[ ... ]`: list
- `{ ... }`: object

## Custom types

Types can reference other types defined in the same directory:

```
// user.ksc
{
	"id": (string),
	"name": (string),
	"age": (int, optional)
}
```

```
// order.ksc
{
	"id": (string),
	"user": (user),
	"price": (number)
}
```

## Examples

```json
// user type
{
  "id": (string),
  "name": (string),
  "email": (string),
  "password": (string(len(x) >= 8)),
  "age": (int, optional)
}
```

The example above defines the `user` type.

- `id` is a *required* string.
- `name` is *required* string
- `email` is a *required* string.
- `password` is a *required* string and constrained to have 8 or more characters.
- `age` is an *optional* int.

