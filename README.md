# stm

[![GoDoc](https://godoc.org/github.com/lukechampine/stm?status.svg)](https://godoc.org/github.com/lukechampine/stm)

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

It remains to be seen whether this style of concurrency has practical
applications in Go. If you find this package useful, please tell me about it!
