package ui

import (
	tea "charm.land/bubbletea/v2"
	tk "github.com/Gandalfalex/tuikit"
	"github.com/aws/aws-sdk-go-v2/service/s3"

	"s3reader/internal/config"
	"s3reader/internal/store"
)

type (
	clientMsg struct {
		tk.Op
		client  *s3.Client
		profile config.Profile
		err     error
	}
	bucketsMsg struct {
		tk.Op
		buckets []store.Entry
		err     error
	}
	bucketOpenedMsg struct {
		tk.Op
		bucket  string
		region  string
		client  *s3.Client
		prefix  string
		entries []store.Entry
		err     error
	}
	dirMsg struct {
		tk.Op
		prefix  string
		entries []store.Entry
		sel     string
		err     error
	}
	searchMsg struct {
		tk.Op
		found   []store.Entry
		scanned int64
		done    bool
		err     error
		ch      <-chan tea.Msg
	}
	infoMsg struct {
		tk.Op
		info store.BucketInfo
	}
	statsMsg struct {
		tk.Op
		stats *store.Stats
		ch    <-chan tea.Msg
	}
	detailMsg struct {
		tk.Op
		detail store.ObjectDetail
	}
)

func waitFor(ch <-chan tea.Msg) tea.Cmd {
	return func() tea.Msg {
		msg, ok := <-ch
		if !ok {
			return nil
		}
		return msg
	}
}
