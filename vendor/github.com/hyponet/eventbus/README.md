# eventbus

EventBus is a very useful tool for event-driven style code. You can use events to chain different logic together without coupling components

## Usage

### Static topic

```go
package main

import (
	"fmt"
	"github.com/hyponet/eventbus"
)

func aAndBComputer(a, b int) {
	fmt.Printf("%d + %d = %d\n", a, b, a+b)
}

func main() {
	eventbus.Subscribe("op.int.and", aAndBComputer)
	eventbus.Publish("op.int.and", 1, 2)
}
```

### Wildcard topic

```go
package main

import (
	"fmt"
	"github.com/hyponet/eventbus"
)

func bobDoSomething() {
	fmt.Println("Bob do something")
}

func aliceDoSomething() {
	fmt.Println("Alice do something")
}

func main() {
	eventbus.Subscribe("partner.bob.do", bobDoSomething)
	eventbus.Subscribe("partner.alice.do", aliceDoSomething)
	eventbus.Publish("partner.*.do")
}
```

## Benchmark

### Publish

```bash
goos: darwin
goarch: amd64
pkg: github.com/hyponet/eventbus
cpu: Intel(R) Core(TM) i7-8700B CPU @ 3.20GHz

BenchmarkBusPublish
BenchmarkBusPublish-2   	   10000	      2954 ns/op	     746 B/op	      15 allocs/op
BenchmarkBusPublish-4   	   10000	      1624 ns/op	     497 B/op	      15 allocs/op
BenchmarkBusPublish-8   	   10000	      1333 ns/op	     497 B/op	      15 allocs/op

BenchmarkBusPublish
BenchmarkBusPublish-2   	  100000	      2369 ns/op	     543 B/op	      15 allocs/op
BenchmarkBusPublish-4   	  100000	      1358 ns/op	     497 B/op	      15 allocs/op
BenchmarkBusPublish-8   	  100000	      1380 ns/op	     497 B/op	      15 allocs/op

BenchmarkBusPublish
BenchmarkBusPublish-2   	 1000000	      2202 ns/op	     504 B/op	      15 allocs/op
BenchmarkBusPublish-4   	 1000000	      1419 ns/op	     497 B/op	      15 allocs/op
BenchmarkBusPublish-8   	 1000000	      1466 ns/op	     497 B/op	      15 allocs/op

BenchmarkBusPublish
BenchmarkBusPublish-2   	10000000	      1992 ns/op	     498 B/op	      15 allocs/op
BenchmarkBusPublish-4   	10000000	      1408 ns/op	     497 B/op	      15 allocs/op
BenchmarkBusPublish-8   	10000000	      1507 ns/op	     497 B/op	      15 allocs/op
```

### Subscribe
```bash
goos: darwin
goarch: amd64
pkg: github.com/hyponet/eventbus
cpu: Intel(R) Core(TM) i7-8700B CPU @ 3.20GHz

BenchmarkBusSubscribe
BenchmarkBusSubscribe-2   	   10000	      2682 ns/op	     962 B/op	      16 allocs/op
BenchmarkBusSubscribe-4   	   10000	      2521 ns/op	     983 B/op	      16 allocs/op
BenchmarkBusSubscribe-8   	   10000	      2762 ns/op	    1196 B/op	      16 allocs/op

BenchmarkBusSubscribe
BenchmarkBusSubscribe-2   	  100000	      2831 ns/op	     951 B/op	      16 allocs/op
BenchmarkBusSubscribe-4   	  100000	      3036 ns/op	     975 B/op	      16 allocs/op
BenchmarkBusSubscribe-8   	  100000	      3066 ns/op	    1087 B/op	      16 allocs/op

BenchmarkBusSubscribe
BenchmarkBusSubscribe-2   	 1000000	      3315 ns/op	    1074 B/op	      16 allocs/op
BenchmarkBusSubscribe-4   	 1000000	      3180 ns/op	    1076 B/op	      16 allocs/op
BenchmarkBusSubscribe-8   	 1000000	      2518 ns/op	     770 B/op	      16 allocs/op

BenchmarkBusSubscribe
BenchmarkBusSubscribe-2   	10000000	      3757 ns/op	    1016 B/op	      16 allocs/op
BenchmarkBusSubscribe-4   	10000000	      3489 ns/op	    1013 B/op	      16 allocs/op
BenchmarkBusSubscribe-8   	10000000	      4136 ns/op	    1256 B/op	      16 allocs/op
```