# stm

## This package is no longer maintained. Please see [anacrolix/stm](https://github.com/anacrolix/stm) for a maintained fork.

[![GoDoc](https://godoc.org/github.com/lukechampine/stm?status.svg)](https://godoc.org/github.com/lukechampine/stm) [![Go Report Card](https://goreportcard.com/badge/github.com/lukechampine/stm)](https://goreportcard.com/report/github.com/lukechampine/stm)

Package `stm` provides [Software Transactional Memory](https://en.wikipedia.org/wiki/Software_transactional_memory) operations for Go. This is
an alternative to the standard way of writing concurrent code (channels and
mutexes). STM makes it easy to perform arbitrarily complex operations in an
atomic fashion. One of its primary advantages over traditional locking is that
STM transactions are composable, whereas locking functions are not -- the
composition will either deadlock or release the lock between functions (making
it non-atomic).

The `stm` API tries to mimic that of Haskell's [`Control.Concurrent.STM`](https://hackage.haskell.org/package/stm-2.4.4.1/docs/Control-Concurrent-STM.html), but
this is not entirely possible due to Go's type system; we are forced to use
`interface{}` and type assertions. Furthermore, Haskell can enforce at compile
time that STM variables are not modified outside the STM monad. This is not
possible in Go, so be especially careful when using pointers in your STM code.
Another significant departure is that `stm.Atomically` does not return a value.
This shortens transaction code a bit, but I'm not 100% it's the right decision.
(The alternative would be for every transaction function to return an `interface{}`.)

Unlike Haskell, data in Go is not immutable by default, which means you have
to be careful when using STM to manage pointers. If two goroutines have access
to the same pointer, it doesn't matter whether they retrieved the pointer
atomically; modifying the pointer can still cause a data race. To resolve
this, either use immutable data structures, or replace pointers with STM
variables. A more concrete example is given below.

It remains to be seen whether this style of concurrency has practical
applications in Go. If you find this package useful, please tell me about it!

## Examples

Here's some example code that demonstrates how to perform common operations:

```go
// create a shared variable
n := stm.NewVar(3)

// read a variable
var v int
stm.Atomically(func(tx *stm.Tx) {
	v = tx.Get(n).(int)
})
// or:
v = stm.AtomicGet(n).(int)

// write to a variable
stm.Atomically(func(tx *stm.Tx) {
	tx.Set(n, 12)
})
// or:
stm.AtomicSet(n, 12)

// update a variable
stm.Atomically(func(tx *stm.Tx) {
	cur := tx.Get(n).(int)
	tx.Set(n, cur-1)
})

// block until a condition is met
stm.Atomically(func(tx *stm.Tx) {
	cur := tx.Get(n).(int)
	if cur != 0 {
		tx.Retry()
	}
	tx.Set(n, 10)
})
// or:
stm.Atomically(func(tx *stm.Tx) {
	cur := tx.Get(n).(int)
	tx.Assert(cur == 0)
	tx.Set(n, 10)
})

// select among multiple (potentially blocking) transactions
stm.Atomically(stm.Select(
	// this function blocks forever, so it will be skipped
	func(tx *stm.Tx) { tx.Retry() },

	// this function will always succeed without blocking
	func(tx *stm.Tx) { tx.Set(n, 10) },

	// this function will never run, because the previous
	// function succeeded
	func(tx *stm.Tx) { tx.Set(n, 11) },
))

// since Select is a normal transaction, if the entire select retries
// (blocks), it will be retried as a whole:
x := 0
stm.Atomically(stm.Select(
	// this function will run twice, and succeed the second time
	func(tx *stm.Tx) { tx.Assert(x == 1) },

	// this function will run once
	func(tx *stm.Tx) {
		x = 1
		tx.Retry()
	},
))
// But wait! Transactions are only retried when one of the Vars they read is
// updated. Since x isn't a stm Var, this code will actually block forever --
// but you get the idea.
```

See [example_santa_test.go](example_santa_test.go) for a more complex example.

## Pointers

Be very careful when managing pointers inside transactions! (This includes
slices, maps, channels, and captured variables.) Here's why:

```go
p := stm.NewVar([]byte{1,2,3})
stm.Atomically(func(tx *stm.Tx) {
	b := tx.Get(p).([]byte)
	b[0] = 7
	tx.Set(p, b)
})
```

This transaction looks innocent enough, but it has a hidden side effect: the
modification of b is visible outside the transaction. Instead of modifying
pointers directly, prefer to operate on immutable values as much as possible.
Following this advice, we can rewrite the transaction to perform a copy:

```go
stm.Atomically(func(tx *stm.Tx) {
	b := tx.Get(p).([]byte)
	c := make([]byte, len(b))
	copy(c, b)
	c[0] = 7
	tx.Set(p, c)
})
```

This is less efficient, but it preserves atomicity.

In the same vein, it would be a mistake to do this:

```go
type foo struct {
	i int
}
p := stm.NewVar(&foo{i: 2})
stm.Atomically(func(tx *stm.Tx) {
	f := tx.Get(p).(*foo)
	f.i = 7
	tx.Set(p, f)
})
```

...because setting `f.i` is a side-effect that escapes the transaction. Here,
the correct approach is to move the `Var` inside the struct:

```go
type foo struct {
	i *stm.Var
}
f := foo{i: stm.NewVar(2)}
stm.Atomically(func(tx *stm.Tx) {
	i := tx.Get(f.i).(int)
	i = 7
	tx.Set(f.i, i)
})
```

## Benchmarks

In synthetic benchmarks, STM seems to have a 1-5x performance penalty compared
to traditional mutex- or channel-based concurrency. However, note that these
benchmarks exhibit a lot of data contention, which is where STM is weakest.
For example, in `BenchmarkIncrementSTM`, each increment transaction retries an
average of 2.5 times. Less contentious benchmarks are forthcoming.

```
BenchmarkAtomicGet-4       	50000000	      26.7 ns/op
BenchmarkAtomicSet-4       	20000000	      65.7 ns/op

BenchmarkIncrementSTM-4    	     500	   2852492 ns/op
BenchmarkIncrementMutex-4  	    2000	    645122 ns/op
BenchmarkIncrementChannel-4	    2000	    986317 ns/op

BenchmarkReadVarSTM-4      	    5000	    268726 ns/op
BenchmarkReadVarMutex-4    	   10000	    248479 ns/op
BenchmarkReadVarChannel-4  	   10000	    240086 ns/op

```
