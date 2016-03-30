# stm

Package `stm` provides [Software Transactional Memory](https://en.wikipedia.org/wiki/Software_transactional_memory) operations for Go. This is
an alternative to the standard way of writing concurrent code (channels and
mutexes). STM makes it easy to perform arbitrarily complex operations in an
atomic fashion. One of its primary advantages over traditional locking is that
STM transactions are composable, whereas locking functions are not -- the
composition will either deadlock or release the lock between functions (making
it non-atomic).

To begin, create an STM object that wraps the data you want to access
concurrently:

```go
	x := stm.NewVar(3)
```
You can then use the Atomically method to atomically read and/or write the the
data. This code atomically decrements `x`:

```go
	stm.Atomically(func(tx *stm.Tx) {
		cur := tx.Get(x).(int)
		tx.Set(x, cur-1)
	})
```

An important part of STM transactions is retrying. At any point during the
transaction, you can call `tx.Retry()`, which will abort the transaction,
but not cancel it entirely. The call to `Atomically` will block until another
call to `Atomically` finishes, at which point the transaction will be rerun.
Specifically, one of the values read by the transaction (via `tx.Get`) must be
updated before the transaction will be rerun. As an example, this code will
try to decrement `x`, but will block as long as `x` is zero:

```go
	stm.Atomically(func(tx *stm.Tx) {
		cur := tx.Get(x).(int)
		if cur == 0 {
			tx.Retry()
		}
		tx.Set(x, cur-1)
	})
```

Internally, `tx.Retry` simply calls `panic(stm.Retry)`. Panicking with any
other value will cancel the transaction; no values will be changed. However,
it is the responsibility of the caller to catch such panics.

Multiple transactions can be composed using `Select`. If the first transaction
calls `Retry`, the next transaction will be run, and so on. If all of the
transactions call `Retry`, the call will block and the entire selection will
be retried. For example, this code implements the "decrement-if-nonzero"
transaction above, but for two values. It will first try to decrement `x`,
then `y`, and block if both values are zero.

```go
	func dec(v *stm.Var) func(*stm.Tx) {
		return func(tx *stm.Tx) {
			cur := tx.Get(v).(int)
			if cur == 0 {
				tx.Retry()
			}
			tx.Set(v, cur-1)
		}
	}

	// Note that Select does not perform any work itself, but merely
	// returns a new transaction.
	stm.Atomically(stm.Select(dec(x), dec(y)))
```

An important caveat: transactions must not have side effects! This is because a
transaction may be restarted several times before completing, meaning the side
effects may execute more than once. This will almost certainly cause incorrect
behavior. One common way to get around this is to build up a list of operations
to perform inside the transaction, and then perform them after the transaction
completes.

The `stm` API tries to mimic that of Haskell's `Control.Concurrent.STM`, but
this is not entirely possible due to Go's type system; we are forced to use
`interface{}` and type assertions. Furthermore, Haskell can enforce at compile
time that STM variables are not modified outside the STM monad. This is not
possible in Go, so be especially careful when using pointers in your STM code.

It remains to be seen whether this style of concurrency has practical
applications in Go. If you find this package useful, please tell me about it!
