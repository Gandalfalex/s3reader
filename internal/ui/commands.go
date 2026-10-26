package ui

import (
	"fmt"
	"strconv"
	"strings"

	tea "charm.land/bubbletea/v2"
)

// command is one ":" ex-command.
type command struct {
	name     string
	aliases  []string
	args     string
	desc     string
	complete func(m *model, arg string) []complItem // candidates for the argument
	run      func(m *model, arg string) tea.Cmd
}

type complItem struct {
	value string
	label string
	desc  string
}

const maxPopup = 10

var commands []command

func init() {
	sortFields := []string{"name", "size", "modified", "name desc", "size desc", "modified desc"}
	queryHints := []complItem{
		{"ext:", "ext:csv,json", "file extension"},
		{"on:", "on:2024-05", "modified in period"},
		{"today", "today", "modified today"},
		{"yesterday", "yesterday", "modified yesterday"},
		{"<7d", "<7d", "newer than 7 days"},
		{">30d", ">30d", "older than 30 days"},
		{">10MB", ">10MB", "bigger than 10 MiB"},
		{"<1KB", "<1KB", "smaller than 1 KiB"},
		{"re:", "re:^log-\\d+", "regular expression"},
		{"class:", "class:glacier", "storage class"},
		{"type:dir", "type:dir", "only directories"},
		{"type:file", "type:file", "only files"},
		{"!", "!tmp", "exclude names containing"},
	}
	completeQuery := func(m *model, arg string) []complItem {
		head, last := "", arg
		if i := strings.LastIndexByte(arg, ' '); i >= 0 {
			head, last = arg[:i+1], arg[i+1:]
		}
		var res []complItem
		for _, h := range matchItems(queryHints, last) {
			h.value = head + h.value
			res = append(res, h)
		}
		return res
	}

	commands = []command{
		{name: "cd", args: "<dir>", desc: "change directory (.., /, a/b/)", complete: completeDirs,
			run: func(m *model, arg string) tea.Cmd { return m.cmdCd(arg) }},
		{name: "open", aliases: []string{"o"}, args: "<name>", desc: "open directory or object", complete: completeEntries,
			run: func(m *model, arg string) tea.Cmd { return m.cmdOpen(arg) }},
		{name: "bucket", aliases: []string{"b"}, args: "<name>", desc: "switch bucket", complete: completeBuckets,
			run: func(m *model, arg string) tea.Cmd { return m.cmdBucket(arg) }},
		{name: "buckets", desc: "list all buckets",
			run: func(m *model, arg string) tea.Cmd { return m.cmdBuckets() }},
		{name: "profile", aliases: []string{"p"}, args: "<name>", desc: "switch profile", complete: completeProfiles,
			run: func(m *model, arg string) tea.Cmd { return m.cmdProfile(arg) }},
		{name: "filter", aliases: []string{"f"}, args: "<query>", desc: "filter current list (empty clears)", complete: completeQuery,
			run: func(m *model, arg string) tea.Cmd { return m.cmdFilter(arg) }},
		{name: "nohlsearch", aliases: []string{"noh"}, desc: "clear filter",
			run: func(m *model, arg string) tea.Cmd { return m.cmdFilter("") }},
		{name: "search", aliases: []string{"s"}, args: "<query>", desc: "recursive search from current dir", complete: completeQuery,
			run: func(m *model, arg string) tea.Cmd { return m.cmdSearch(arg) }},
		{name: "sort", args: "<field> [desc]", desc: "sort by name, size, modified",
			complete: func(m *model, arg string) []complItem { return matchStrings(sortFields, arg) },
			run:      func(m *model, arg string) tea.Cmd { return m.cmdSort(arg) }},
		{name: "info", aliases: []string{"i"}, desc: "bucket info",
			run: func(m *model, arg string) tea.Cmd { return m.cmdInfo(nil) }},
		{name: "stats", args: "[dir]", desc: "scan objects: count, size, classes (/ = whole bucket)", complete: completeDirs,
			run: func(m *model, arg string) tea.Cmd { p := m.resolveDir(arg); return m.cmdInfo(&p) }},
		{name: "yank", aliases: []string{"y"}, desc: "copy s3:// URI of current item",
			run: func(m *model, arg string) tea.Cmd { return m.cmdYank() }},
		{name: "reload", aliases: []string{"e", "edit"}, desc: "reload current view",
			run: func(m *model, arg string) tea.Cmd { m.popToList(); _, c := m.reload(); return c }},
		{name: "messages", aliases: []string{"errors", "mes"}, desc: "error log",
			run: func(m *model, arg string) tea.Cmd { m.openErrors(); return nil }},
		{name: "help", aliases: []string{"h"}, desc: "key bindings and query syntax",
			run: func(m *model, arg string) tea.Cmd {
				m.openPage(scrHelp, false)
				return nil
			}},
		{name: "quit", aliases: []string{"q", "q!", "qa", "qa!", "x"}, desc: "quit",
			run: func(m *model, arg string) tea.Cmd { return tea.Quit }},
	}
}

// matchItems keeps items whose value starts with prefix (case-insensitive),
// followed by items that merely contain it.
func matchItems(items []complItem, prefix string) []complItem {
	p := strings.ToLower(prefix)
	var starts, contains []complItem
	for _, it := range items {
		v := strings.ToLower(it.value)
		switch {
		case strings.HasPrefix(v, p):
			starts = append(starts, it)
		case p != "" && strings.Contains(v, p):
			contains = append(contains, it)
		}
	}
	return append(starts, contains...)
}

func matchStrings(vals []string, prefix string) []complItem {
	items := make([]complItem, len(vals))
	for i, v := range vals {
		items[i] = complItem{value: v, label: v}
	}
	return matchItems(items, prefix)
}

func resolveCommand(name string) (*command, error) {
	for i := range commands {
		c := &commands[i]
		if c.name == name {
			return c, nil
		}
		for _, a := range c.aliases {
			if a == name {
				return c, nil
			}
		}
	}
	var hits []*command
	for i := range commands {
		if strings.HasPrefix(commands[i].name, name) {
			hits = append(hits, &commands[i])
		}
	}
	switch len(hits) {
	case 1:
		return hits[0], nil
	case 0:
		return nil, fmt.Errorf("E492: Not an editor command: %s", name)
	}
	names := make([]string, len(hits))
	for i, h := range hits {
		names[i] = h.name
	}
	return nil, fmt.Errorf("E464: Ambiguous use of command: %s (%s)", name, strings.Join(names, ", "))
}

func completeDirs(m *model, arg string) []complItem {
	items := []complItem{{"..", "..", "parent directory"}, {"/", "/", "bucket root"}}
	// complete one level below what is typed, using the loaded listing for the
	// current directory only
	if !strings.Contains(strings.TrimSuffix(arg, "/"), "/") {
		for _, e := range m.objects.all {
			if e.IsDir {
				items = append(items, complItem{value: e.Name, label: e.Name, desc: "dir"})
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
		items = append(items, complItem{value: e.Name, label: e.Name, desc: desc})
	}
	return matchItems(items, arg)
}

func completeBuckets(m *model, arg string) []complItem {
	var items []complItem
	for _, b := range m.buckets.all {
		items = append(items, complItem{value: b.Name, label: b.Name, desc: "created " + fmtTime(b.Modified)})
	}
	if m.bucket != "" && len(items) == 0 {
		items = append(items, complItem{value: m.bucket, label: m.bucket, desc: "current (:buckets to list all)"})
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
		items = append(items, complItem{value: p.Name, label: p.Name, desc: desc})
	}
	return matchItems(items, arg)
}

// computeCompletions builds the popup for the current command-line text.
func (m *model) computeCompletions(input string) []complItem {
	name, arg, hasArg := strings.Cut(strings.TrimLeft(input, " "), " ")
	if !hasArg {
		if _, err := strconv.Atoi(name); err == nil {
			return []complItem{{value: name, label: name, desc: "jump to line " + name}}
		}
		var items []complItem
		for _, c := range commands {
			label := c.name
			if len(c.aliases) > 0 {
				label += " (" + strings.Join(c.aliases, ",") + ")"
			}
			if c.args != "" {
				label += " " + c.args
			}
			items = append(items, complItem{value: c.name, label: label, desc: c.desc})
			for _, a := range c.aliases {
				if a == name { // exact alias typed: surface its command first
					items = append([]complItem{{value: c.name, label: label, desc: c.desc}}, items[:len(items)-1]...)
				}
			}
		}
		return matchItems(items, name)
	}
	c, err := resolveCommand(name)
	if err != nil || c.complete == nil {
		return nil
	}
	items := c.complete(m, arg)
	for i := range items {
		items[i].value = name + " " + items[i].value
	}
	return items
}
