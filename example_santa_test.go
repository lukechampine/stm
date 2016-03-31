// An implementation of the "Santa Claus problem" as defined in 'Beautiful
// concurrency', found here: http://research.microsoft.com/en-us/um/people/simonpj/papers/stm/beautiful.pdf
//
// The problem is given as:
//
//   Santa repeatedly sleeps until wakened by either all of his nine reindeer,
//   back from their holidays, or by a group of three of his ten elves. If
//   awakened by the reindeer, he harnesses each of them to his sleigh,
//   delivers toys with them and finally unharnesses them (allowing them to
//   go off on holiday). If awakened by a group of elves, he shows each of the
//   group into his study, consults with them on toy R&D and finally shows
//   them each out (allowing them to go back to work). Santa should give
//   priority to the reindeer in the case that there is both a group of elves
//   and a group of reindeer waiting.
//
// Here we follow the solution given in the paper, described as such:
//
//   Santa makes one "Group" for the elves and one for the reindeer. Each elf
//   (or reindeer) tries to join its Group. If it succeeds, it gets two
//   "Gates" in return. The first Gate allows Santa to control when the elf
//   can enter the study, and also lets Santa know when they are all inside.
//   Similarly, the second Gate controls the elves leaving the study. Santa,
//   for his part, waits for either of his two Groups to be ready, and then
//   uses that Group's Gates to marshal his helpers (elves or reindeer)
//   through their task. Thus the helpers spend their lives in an infinite
//   loop: try to join a group, move through the gates under Santa's control,
//   and then delay for a random interval before trying to join a group again.
//
// See the paper for more details regarding the solution's implementation.
package stm_test

import (
	"fmt"
	"math/rand"
	"time"

	"github.com/lukechampine/stm"
)

type gate struct {
	capacity  int
	remaining *stm.Var
}

func (g *gate) pass() {
	stm.Atomically(func(tx *stm.Tx) {
		rem := tx.Get(g.remaining).(int)
		// wait until gate can hold us
		tx.Assert(rem > 0)
		tx.Set(g.remaining, rem-1)
	})
}

func (g *gate) operate() {
	// open gate, reseting capacity
	stm.AtomicSet(g.remaining, g.capacity)
	// wait for gate to be full
	stm.Atomically(func(tx *stm.Tx) {
		rem := tx.Get(g.remaining).(int)
		tx.Assert(rem == 0)
	})
}

func newGate(capacity int) *gate {
	return &gate{
		capacity:  capacity,
		remaining: stm.NewVar(0), // gate starts out closed
	}
}

type group struct {
	capacity     int
	remaining    *stm.Var
	gate1, gate2 *stm.Var
}

func newGroup(capacity int) *group {
	return &group{
		capacity:  capacity,
		remaining: stm.NewVar(capacity), // group starts out with full capacity
		gate1:     stm.NewVar(newGate(capacity)),
		gate2:     stm.NewVar(newGate(capacity)),
	}
}

func (g *group) join() (g1, g2 *gate) {
	stm.Atomically(func(tx *stm.Tx) {
		rem := tx.Get(g.remaining).(int)
		// wait until the group can hold us
		tx.Assert(rem > 0)
		tx.Set(g.remaining, rem-1)
		// return the group's gates
		g1 = tx.Get(g.gate1).(*gate)
		g2 = tx.Get(g.gate2).(*gate)
	})
	return
}

func (g *group) await(tx *stm.Tx) (*gate, *gate) {
	// wait for group to be empty
	rem := tx.Get(g.remaining).(int)
	tx.Assert(rem == 0)
	// get the group's gates
	g1 := tx.Get(g.gate1).(*gate)
	g2 := tx.Get(g.gate2).(*gate)
	// reset group
	tx.Set(g.remaining, g.capacity)
	tx.Set(g.gate1, newGate(g.capacity))
	tx.Set(g.gate2, newGate(g.capacity))
	return g1, g2
}

func spawnElf(g *group, id int) {
	for {
		in, out := g.join()
		in.pass()
		fmt.Printf("Elf %v meeting in the study\n", id)
		out.pass()
		// sleep for a random interval <5s
		time.Sleep(time.Duration(rand.Intn(5000)) * time.Millisecond)
	}
}

func spawnReindeer(g *group, id int) {
	for {
		in, out := g.join()
		in.pass()
		fmt.Printf("Reindeer %v delivering toys\n", id)
		out.pass()
		// sleep for a random interval <5s
		time.Sleep(time.Duration(rand.Intn(5000)) * time.Millisecond)
	}
}

type selection struct {
	task         string
	gate1, gate2 *gate
}

func chooseGroup(g *group, task string, s *selection) func(*stm.Tx) {
	return func(tx *stm.Tx) {
		s.gate1, s.gate2 = g.await(tx)
		s.task = task
	}
}

func spawnSanta(elves, reindeer *group) {
	for {
		fmt.Println("-------------")
		var s selection
		stm.Atomically(stm.Select(
			// prefer reindeer to elves
			chooseGroup(reindeer, "deliver toys", &s),
			chooseGroup(elves, "meet in my study", &s),
		))
		fmt.Printf("Ho! Ho! Ho! Let's %s!\n", s.task)
		s.gate1.operate()
		// helpers do their work here...
		s.gate2.operate()
	}
}

func Example() {
	elfGroup := newGroup(3)
	for i := 0; i < 10; i++ {
		go spawnElf(elfGroup, i)
	}
	reinGroup := newGroup(9)
	for i := 0; i < 9; i++ {
		go spawnReindeer(reinGroup, i)
	}
	// blocks forever
	spawnSanta(elfGroup, reinGroup)
}
