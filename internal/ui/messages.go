package ui

import (
	tea "charm.land/bubbletea/v2"
	"github.com/aws/aws-sdk-go-v2/service/s3"

	"s3reader/internal/config"
	"s3reader/internal/store"
)

// op tags async results with the generation of the operation that produced
// them; Update drops results from superseded or cancelled operations.
type op int

func (o op) opGen() int { return int(o) }

type opResult interface{ opGen() int }

type (
	clientMsg struct {
		op
		client  *s3.Client
		profile config.Profile
		err     error
	}
	bucketsMsg struct {
		op
		buckets []store.Entry
		err     error
	}
	bucketOpenedMsg struct {
		op
		bucket  string
		region  string
		client  *s3.Client
		prefix  string
		entries []store.Entry
		err     error
	}
	dirMsg struct {
		op
		prefix  string
		entries []store.Entry
		sel     string
		err     error
	}
	searchMsg struct {
		op
		found   []store.Entry
		scanned int64
		done    bool
		err     error
		ch      <-chan tea.Msg
	}
	infoMsg struct {
		op
		info store.BucketInfo
	}
	statsMsg struct {
		op
		stats *store.Stats
		ch    <-chan tea.Msg
	}
	detailMsg struct {
		op
		detail store.ObjectDetail
	}
	statusMsg string
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
