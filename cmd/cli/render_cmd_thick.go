//go:build thick
// +build thick

package main

func init() {
	renderCmdFlagSet.BoolP(
		flagDebug,
		"d",
		false,
		"display debug output",
	)
}
