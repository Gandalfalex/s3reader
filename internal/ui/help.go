package ui

import (
	"sync"

	tk "github.com/Gandalfalex/tuikit"
)

// helpPage is built once: static key help plus the generated command list.
var helpPage = sync.OnceValue(func() string {
	return "\n" + tk.MotionHelp + "\n" + helpText + "\n  COMMANDS (:, tab completes, ↑↓ history, unique prefixes work: :buc, :prof)\n" +
		tk.CommandHelp(commands) + "    :42                       jump to line 42\n"
})

const helpText = `  NAVIGATION
    l / enter / →       open profile, bucket, directory or object
    h / - / ← / bksp    go up a directory / back
    gr                  jump to bucket root
    ctrl+o / ctrl+i     jump back / forward through visited dirs and buckets
    q                   quit (press twice inside a dir; back, in pager views)
    ctrl+c              quit anywhere
    esc                 clear filter → cancel request → back

  FILTER, SEARCH, SORT
    /                   live filter current list (enter keeps, esc reverts)
    ? (or s)            recursive search from current dir
    D                   (search results) reveal object in its directory
    o / O               cycle sort (name, size, modified) / reverse
    r                   reload (dir listings are cached for 5 min)

  INFO
    i / gi              bucket info (versioning, retention, encryption, ...)
    c                   (info) scan whole bucket: count, size, classes, ext
    C                   (info) scan only the current prefix
    y / Y               yank s3:// URI / raw key to clipboard
    E                   error log (this session; full log in log file)

  QUERY SYNTAX (filter and search; terms are ANDed)
    foo                 name contains foo (smart-case)
    !foo                name does not contain foo
    re:^log-\d+         regular expression
    ext:csv,json        extension
    >10MB  <1G          size bounds (K, M, G, T)
    class:glacier       storage class
    type:dir type:file  only directories / only files

  DATES (last modified; a date means its whole period)
    2024  2024-05  2024-05-17  17.05.2024  2024-05-17T14:30  today  yesterday
    2024-05             name contains "2024-05" OR modified in May 2024
    on:2024-05          only modified in May 2024 (also date:)
    >2024-05  <2025     after May 2024 / before 2025
    >=2024-05 <=2024-06 same, but including the named period
    2024-01..2024-03    Jan through Mar 2024 (2024-01.. or ..2023 open-ended)
    <7d  >2w            newer than 7 days / older than 2 weeks (h d w mo y)
    after:X before:X    same as >X <X; after:7d = newer than 7 days

  EXAMPLE
    ?  ext:parquet >100MB <30d !tmp
    /  2024-05..2024-06 ext:csv
`
