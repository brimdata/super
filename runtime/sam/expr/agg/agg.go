package agg

// MaxValueSize limits the size of a value produced by an aggregate function
// since sets and arrays could otherwise grow without bound.
var MaxValueSize = 1024 * 1024 * 1024
