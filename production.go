package grete

import "github.com/ccbhj/grete/internal/rete"

type Production struct {
	Then func()
	Desc string
	rete.Production
}
