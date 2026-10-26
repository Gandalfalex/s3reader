package ui

type screen int

const (
	scrProfiles screen = iota
	scrBuckets
	scrObjects
	scrSearch
	scrInfo
	scrObject
	scrHelp
	scrErrors
)

type inputMode int

const (
	inputNone inputMode = iota
	inputFilter
	inputSearch
	inputCommand
)
