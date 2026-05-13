// Package main — CLI-обёртка над goose для миграций PostgreSQL.
//
// На День 1 — заглушка. Реальная реализация появится в Дне 3
// (см. execution_plan.md, раздел 21.4).
package main

import (
	"fmt"
	"os"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("usage: migrate up|down|status|...")
		os.Exit(1)
	}
	fmt.Printf("max-university-event-bot: migrate %s (stub, Day 1)\n", os.Args[1])
}
