package ui

import (
	"encoding/hex"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	tea "charm.land/bubbletea/v2"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"

	"s3reader/internal/store"
)

// refreshViewport rebuilds content for pager screens.
func (m *model) refreshViewport() {
	switch m.top() {
	case scrInfo:
		m.vp.SoftWrap = false
		m.vp.SetContent(m.renderInfo())
	case scrObject:
		m.vp.SoftWrap = true
		m.vp.SetContent(m.renderDetail())
	case scrHelp:
		m.vp.SoftWrap = false
		m.vp.SetContent(helpPage())
	case scrErrors:
		m.vp.SoftWrap = true
		m.vp.SetContent(m.renderErrors())
	}
}

func (m *model) openErrors() {
	m.openPage(scrErrors, true)
}

// openPage pushes a pager screen and renders its content.
func (m *model) openPage(s screen, atBottom bool) {
	m.push(s)
	m.refreshViewport()
	if atBottom {
		m.vp.GotoBottom()
	} else {
		m.vp.GotoTop()
	}
}

// scanStats starts an object scan for the bucket shown in the info page.
func (m *model) scanStats(prefix string) tea.Cmd {
	cmd := m.startStats(m.info.Name, prefix)
	m.refreshViewport()
	return cmd
}

func (m *model) renderErrors() string {
	if len(m.errs) == 0 {
		return "\n" + sMuted.Render("  no errors this session") + "\n"
	}
	var b strings.Builder
	b.WriteString("\n")
	for _, e := range m.errs {
		b.WriteString("  " + sMuted.Render(e.at.Format("15:04:05")) + "  " + sWarn.Render(e.what) + "\n")
		b.WriteString("    " + sErr.Render(e.err.Error()) + "\n\n")
	}
	return b.String()
}

func row(label, value string) string {
	return "  " + sLabel.Render(label) + value + "\n"
}

func (m *model) renderInfo() string {
	bi := m.info
	if bi == nil {
		return ""
	}
	var b strings.Builder
	b.WriteString("\n" + sSection.Render(" BUCKET") + "\n")
	b.WriteString(row("Name", bi.Name))
	b.WriteString(row("Region", bi.Region))
	b.WriteString(row("Endpoint", bi.Endpoint))
	if !bi.Created.IsZero() {
		b.WriteString(row("Created", fmtTime(bi.Created)+sMuted.Render("  ("+ago(bi.Created)+")")))
	}
	if bi.Owner != "" {
		b.WriteString(row("Owner", bi.Owner))
	}

	b.WriteString("\n" + sSection.Render(" CONFIGURATION") + "\n")
	ver := bi.Versioning
	if ver == "Enabled" {
		ver = sOK.Render(ver)
	}
	if bi.MFADelete != "" {
		ver += sMuted.Render("  MFA delete: " + bi.MFADelete)
	}
	b.WriteString(row("Versioning", ver))
	enc := strings.Join(bi.Encryption, ", ")
	if enc == "" {
		enc = "none"
	}
	b.WriteString(row("Encryption", enc))
	b.WriteString(row("Public access", bi.PublicBlock))
	pol := bi.PolicyState
	if pol == "PUBLIC" {
		pol = sErr.Render(pol)
	}
	b.WriteString(row("Bucket policy", pol))
	if len(bi.Lifecycle) == 0 {
		b.WriteString(row("Lifecycle", "none"))
	} else {
		b.WriteString(row("Lifecycle", fmt.Sprintf("%d rule(s)", len(bi.Lifecycle))))
		for _, r := range bi.Lifecycle {
			b.WriteString(row("", sMuted.Render("• ")+r))
		}
	}
	if len(bi.Tags) > 0 {
		b.WriteString("\n" + sSection.Render(" TAGS") + "\n")
		for _, t := range bi.Tags {
			b.WriteString(row(t[0], t[1]))
		}
	}

	b.WriteString("\n" + sSection.Render(" CONTENTS") + "\n")
	st := m.stats
	if st == nil {
		b.WriteString(sMuted.Render("  press c to scan all objects (count, size, classes, extensions)") + "\n")
		if m.info.Name == m.bucket && m.prefix != "" {
			b.WriteString(sMuted.Render("  press C to scan only s3://"+m.bucket+"/"+m.prefix) + "\n")
		}
		return b.String()
	}
	scope := "s3://" + bi.Name + "/" + st.Prefix
	state := m.spin.View() + " scanning…"
	if st.Done {
		state = sOK.Render("✓ complete") + sMuted.Render(fmt.Sprintf(" in %s", st.Finished.Sub(st.Started).Round(time.Millisecond)))
		if st.Err != nil {
			state = sErr.Render("error: " + st.Err.Error())
		}
	}
	b.WriteString(row("Scope", scope))
	b.WriteString(row("Status", state))
	b.WriteString(row("Objects", fmtInt(st.Count)))
	b.WriteString(row("Total size", fmt.Sprintf("%s  %s", humanSize(st.Size), sMuted.Render("("+fmtInt(st.Size)+" bytes)"))))
	if st.Count > 0 {
		b.WriteString(row("Average size", humanSize(st.Size/st.Count)))
		b.WriteString(row("Largest", humanSize(st.Largest.Size)+"  "+sMuted.Render(st.Largest.Key)))
		b.WriteString(row("Newest", fmtTime(st.Newest.Modified)+"  "+sMuted.Render(st.Newest.Key)))
		b.WriteString(row("Oldest", fmtTime(st.Oldest.Modified)+"  "+sMuted.Render(st.Oldest.Key)))

		b.WriteString("\n" + sSection.Render(" STORAGE CLASSES") + "\n")
		for _, c := range store.TopBy(st.ByClass, 20) {
			b.WriteString(row(c.K, statLine(c.V, st.Size)))
		}
		b.WriteString("\n" + sSection.Render(" TOP EXTENSIONS (by size)") + "\n")
		for _, c := range store.TopBy(st.ByExt, 15) {
			b.WriteString(row(c.K, statLine(c.V, st.Size)))
		}
	}
	return b.String()
}

func statLine(v [2]int64, total int64) string {
	pct := 0.0
	if total > 0 {
		pct = float64(v[1]) / float64(total) * 100
	}
	barW := 20
	filled := int(pct / 100 * float64(barW))
	bar := sOK.Render(strings.Repeat("█", filled)) + sMuted.Render(strings.Repeat("░", barW-filled))
	return fmt.Sprintf("%s %5.1f%%  %10s  %s objects", bar, pct, humanSize(v[1]), fmtInt(v[0]))
}

func (m *model) renderDetail() string {
	d := m.detail
	if d == nil || d.Head == nil {
		return ""
	}
	h := d.Head
	var b strings.Builder
	b.WriteString("\n" + sSection.Render(" OBJECT") + "\n")
	b.WriteString(row("Key", d.Key))
	b.WriteString(row("URI", "s3://"+m.bucket+"/"+d.Key))
	size := aws.ToInt64(h.ContentLength)
	b.WriteString(row("Size", fmt.Sprintf("%s  %s", humanSize(size), sMuted.Render("("+fmtInt(size)+" bytes)"))))
	if h.LastModified != nil {
		b.WriteString(row("Last modified", fmtTime(*h.LastModified)+sMuted.Render("  ("+ago(*h.LastModified)+")")))
	}
	opt := func(label string, v *string) {
		if s := aws.ToString(v); s != "" {
			b.WriteString(row(label, s))
		}
	}
	opt("Content-Type", h.ContentType)
	opt("Content-Encoding", h.ContentEncoding)
	opt("Content-Language", h.ContentLanguage)
	opt("Content-Disp.", h.ContentDisposition)
	opt("Cache-Control", h.CacheControl)
	b.WriteString(row("ETag", strings.Trim(aws.ToString(h.ETag), `"`)))
	cls := string(h.StorageClass)
	if cls == "" {
		cls = "STANDARD"
	}
	b.WriteString(row("Storage class", cls))
	opt("Version ID", h.VersionId)
	if h.ServerSideEncryption != "" {
		b.WriteString(row("Encryption", string(h.ServerSideEncryption)))
	}
	opt("KMS key", h.SSEKMSKeyId)
	opt("Restore", h.Restore)
	if h.ObjectLockMode != "" {
		b.WriteString(row("Object lock", string(h.ObjectLockMode)))
	}
	if aws.ToInt32(h.PartsCount) > 0 {
		b.WriteString(row("Parts", strconv.Itoa(int(aws.ToInt32(h.PartsCount)))))
	}
	opt("Checksum CRC32", h.ChecksumCRC32)
	opt("Checksum SHA256", h.ChecksumSHA256)

	if len(h.Metadata) > 0 {
		b.WriteString("\n" + sSection.Render(" USER METADATA") + "\n")
		keys := make([]string, 0, len(h.Metadata))
		for k := range h.Metadata {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			b.WriteString(row(k, h.Metadata[k]))
		}
	}
	if len(d.Tags) > 0 {
		b.WriteString("\n" + sSection.Render(" TAGS") + "\n")
		for _, t := range d.Tags {
			b.WriteString(row(t[0], t[1]))
		}
	}

	b.WriteString("\n" + sSection.Render(" PREVIEW") + "\n")
	switch {
	case d.Err != nil:
		b.WriteString("  " + sErr.Render(d.Err.Error()) + "\n")
	case h.StorageClass == types.StorageClassGlacier || h.StorageClass == types.StorageClassDeepArchive:
		b.WriteString(sMuted.Render("  archived object — restore required to read") + "\n")
	case len(d.Preview) == 0:
		b.WriteString(sMuted.Render("  (empty)") + "\n")
	case isText(d.Preview):
		b.WriteString(sMuted.Render(strings.Repeat("─", max(0, m.w-1))) + "\n")
		b.WriteString(sanitize(string(d.Preview)))
		b.WriteString("\n")
		if size > int64(len(d.Preview)) {
			b.WriteString(sMuted.Render(fmt.Sprintf("── truncated: showing first %s of %s ──", humanSize(int64(len(d.Preview))), humanSize(size))) + "\n")
		}
	default:
		n := min(len(d.Preview), 4096)
		b.WriteString(sMuted.Render(fmt.Sprintf("  binary — hex dump of first %d bytes", n)) + "\n\n")
		b.WriteString(hex.Dump(d.Preview[:n]))
	}
	return b.String()
}

func isText(b []byte) bool {
	// allow a split rune at the end of a ranged read
	for i := 0; i < 4 && !utf8.Valid(b) && len(b) > 0; i++ {
		b = b[:len(b)-1]
	}
	if !utf8.Valid(b) {
		return false
	}
	ctrl := 0
	for _, c := range b {
		if c == 0 {
			return false
		}
		if c < 32 && c != '\n' && c != '\r' && c != '\t' {
			ctrl++
		}
	}
	return ctrl*100 < len(b)+1
}

func sanitize(s string) string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.ReplaceAll(s, "\t", "    ")
	return strings.Map(func(r rune) rune {
		if r == '\n' {
			return r
		}
		if r < 32 || r == 0x7f || r == utf8.RuneError {
			return '·'
		}
		return r
	}, s)
}
