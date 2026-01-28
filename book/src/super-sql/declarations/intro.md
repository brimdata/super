# Declarations

Declarations bind a name in the form of an [identifier](../queries.md#identifiers)
to various entities and may appear at the beginning of any [scope](../queries.md#scope)
including the main scope.

Declarations may be created for
* [constants](constants.md),
* [types](types.md),
* [queries](queries.md),
* [functions](functions.md),
* [operators](operators.md), or
* [pragmas](pragmas.md).

With the exception of types that reference other types, all of the names defined in a given scope are available to other declarations defined
in the same scope (as well as containing scopes) independent of the order of declaration,
i.e., a declaration may forward-reference another declaration that is defined in the
same scope.

A declaration may override another declaration of the same name in a parent scope,
but declarations in the same scope with the same name conflict and result in an error.
