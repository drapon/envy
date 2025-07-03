package main

import (
	"github.com/drapon/envy/cmd/root"

	// Import all commands to register them
	_ "github.com/drapon/envy/cmd/cache"
	_ "github.com/drapon/envy/cmd/configure"
	_ "github.com/drapon/envy/cmd/diff"
	_ "github.com/drapon/envy/cmd/export"
	_ "github.com/drapon/envy/cmd/init"
	_ "github.com/drapon/envy/cmd/list"
	_ "github.com/drapon/envy/cmd/pull"
	_ "github.com/drapon/envy/cmd/push"
	_ "github.com/drapon/envy/cmd/run"
	_ "github.com/drapon/envy/cmd/validate"
	_ "github.com/drapon/envy/cmd/version"
)

func main() {
	root.Execute()
}
