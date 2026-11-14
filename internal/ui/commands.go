package ui

import (
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/Gandalfalex/vim-kit/cmdline"
)

// Commands, lookup and completion come from vimkit/cmdline; this file holds
// the command list and the argument completers.
type (
	command   = cmdline.Command[*model, tea.Cmd]
	complItem = cmdline.Item
)

const maxPopup = 10

var commands cmdline.Set[*model, tea.Cmd]

func init() {
	sortFields := []string{"name", "size", "modified", "name desc", "size desc", "modified desc"}
	queryHints := []complItem{
		{Value: "ext:", Label: "ext:csv,json", Desc: "file extension"},
		{Value: "on:", Label: "on:2024-05", Desc: "modified in period"},
		{Value: "today", Label: "today", Desc: "modified today"},
		{Value: "yesterday", Label: "yesterday", Desc: "modified yesterday"},
		{Value: "<7d", Label: "<7d", Desc: "newer than 7 days"},
		{Value: ">30d", Label: ">30d", Desc: "older than 30 days"},
		{Value: ">10MB", Label: ">10MB", Desc: "bigger than 10 MiB"},
		{Value: "<1KB", Label: "<1KB", Desc: "smaller than 1 KiB"},
		{Value: "re:", Label: "re:^log-\\d+", Desc: "regular expression"},
		{Value: "class:", Label: "class:glacier", Desc: "storage class"},
		{Value: "type:dir", Label: "type:dir", Desc: "only directories"},
		{Value: "type:file", Label: "type:file", Desc: "only files"},
		{Value: "!", Label: "!tmp", Desc: "exclude names containing"},
	}
	completeQuery := func(m *model, arg string) []complItem {
		head, last := "", arg
		if i := strings.LastIndexByte(arg, ' '); i >= 0 {
			head, last = arg[:i+1], arg[i+1:]
		}
		var res []complItem
		for _, h := range matchItems(queryHints, last) {
			h.Value = head + h.Value
			res = append(res, h)
		}
		return res
	}

	commands = cmdline.Set[*model, tea.Cmd]{
		{Name: "cd", Args: "<dir>", Desc: "change directory (.., /, a/b/)", Complete: completeDirs,
			Run: func(m *model, arg string) tea.Cmd { return m.cmdCd(arg) }},
		{Name: "open", Aliases: []string{"o"}, Args: "<name>", Desc: "open directory or object", Complete: completeEntries,
			Run: func(m *model, arg string) tea.Cmd { return m.cmdOpen(arg) }},
		{Name: "bucket", Aliases: []string{"b"}, Args: "<name>", Desc: "switch bucket", Complete: completeBuckets,
			Run: func(m *model, arg string) tea.Cmd { return m.cmdBucket(arg) }},
		{Name: "buckets", Desc: "list all buckets",
			Run: func(m *model, arg string) tea.Cmd { return m.cmdBuckets() }},
		{Name: "profile", Aliases: []string{"p"}, Args: "<name>", Desc: "switch profile", Complete: completeProfiles,
			Run: func(m *model, arg string) tea.Cmd { return m.cmdProfile(arg) }},
		{Name: "filter", Aliases: []string{"f"}, Args: "<query>", Desc: "filter current list (empty clears)", Complete: completeQuery,
			Run: func(m *model, arg string) tea.Cmd { return m.cmdFilter(arg) }},
		{Name: "nohlsearch", Aliases: []string{"noh"}, Desc: "clear filter",
			Run: func(m *model, arg string) tea.Cmd { return m.cmdFilter("") }},
		{Name: "search", Aliases: []string{"s"}, Args: "<query>", Desc: "recursive search from current dir", Complete: completeQuery,
			Run: func(m *model, arg string) tea.Cmd { return m.cmdSearch(arg) }},
		{Name: "sort", Args: "<field> [desc]", Desc: "sort by name, size, modified",
			Complete: func(m *model, arg string) []complItem { return matchStrings(sortFields, arg) },
			Run:      func(m *model, arg string) tea.Cmd { return m.cmdSort(arg) }},
		{Name: "info", Aliases: []string{"i"}, Desc: "bucket info",
			Run: func(m *model, arg string) tea.Cmd { return m.cmdInfo(nil) }},
		{Name: "stats", Args: "[dir]", Desc: "scan objects: count, size, classes (/ = whole bucket)", Complete: completeDirs,
			Run: func(m *model, arg string) tea.Cmd { p := m.resolveDir(arg); return m.cmdInfo(&p) }},
		{Name: "yank", Aliases: []string{"y"}, Desc: "copy s3:// URI of current item",
			Run: func(m *model, arg string) tea.Cmd { return m.cmdYank() }},
		{Name: "reload", Aliases: []string{"e", "edit"}, Desc: "reload current view",
			Run: func(m *model, arg string) tea.Cmd { m.popToList(); _, c := m.reload(); return c }},
		{Name: "messages", Aliases: []string{"errors", "mes"}, Desc: "error log",
			Run: func(m *model, arg string) tea.Cmd { m.openErrors(); return nil }},
		{Name: "help", Aliases: []string{"h"}, Desc: "key bindings and query syntax",
			Run: func(m *model, arg string) tea.Cmd {
				m.openPage(scrHelp, false)
				return nil
			}},
		{Name: "quit", Aliases: []string{"q", "q!", "qa", "qa!", "x"}, Desc: "quit",
			Run: func(m *model, arg string) tea.Cmd { return tea.Quit }},
	}
}

var (
	matchItems   = cmdline.Match
	matchStrings = cmdline.MatchStrings
)

func completeDirs(m *model, arg string) []complItem {
	items := []complItem{{Value: "..", Label: "..", Desc: "parent directory"}, {Value: "/", Label: "/", Desc: "bucket root"}}
	// complete one level below what is typed, using the loaded listing for the
	// current directory only
	if !strings.Contains(strings.TrimSuffix(arg, "/"), "/") {
		for _, e := range m.objects.all {
			if e.IsDir {
				items = append(items, complItem{Value: e.Name, Label: e.Name, Desc: "dir"})
			}
		}
	}
	return matchItems(items, arg)
}

func completeEntries(m *model, arg string) []complItem {
	var items []complItem
	for _, e := range m.objects.all {
		desc := humanSize(e.Size) + "  " + fmtTime(e.Modified)
		if e.IsDir {
			desc = "dir"
		}
		items = append(items, complItem{Value: e.Name, Label: e.Name, Desc: desc})
	}
	return matchItems(items, arg)
}

func completeBuckets(m *model, arg string) []complItem {
	var items []complItem
	for _, b := range m.buckets.all {
		items = append(items, complItem{Value: b.Name, Label: b.Name, Desc: "created " + fmtTime(b.Modified)})
	}
	if m.bucket != "" && len(items) == 0 {
		items = append(items, complItem{Value: m.bucket, Label: m.bucket, Desc: "current (:buckets to list all)"})
	}
	return matchItems(items, arg)
}

func completeProfiles(m *model, arg string) []complItem {
	var items []complItem
	for _, p := range m.settings.Profiles {
		desc := p.Endpoint
		if desc == "" {
			desc = "aws " + p.Region
		}
		items = append(items, complItem{Value: p.Name, Label: p.Name, Desc: desc})
	}
	return matchItems(items, arg)
}

// computeCompletions builds the popup for the current command-line text.
func (m *model) computeCompletions(input string) []complItem {
	return commands.Complete(m, input)
}
